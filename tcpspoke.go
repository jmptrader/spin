package spin

import (
	"errors"
	"net"
)

type TCPSpoke struct {
	id   Id
	conn *net.TCPConn
}

func (spoke *TCPSpoke) Id() Id { return spoke.id }
func (spoke *TCPSpoke) Receive() ([]byte, bool) {
	for {
		msg, err := readmsg(spoke.conn)
		if err != nil {
			return nil, false
		}
		if msg.leave {
			spoke.Leave()
			return nil, false
		}
		if !msg.ignore {
			return msg.data, true
		}
	}
}
func (spoke *TCPSpoke) Leave() {
	write(spoke.conn, []byte{leaveByte})
	spoke.conn.Close()
}
func (spoke *TCPSpoke) Feedback(data []byte) {
	writemsg(spoke.conn, data)
}

func Dial(addr string, hubId Id, param []byte) (Spoke, error) {
	connu, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	var success int
	conn := connu.(*net.TCPConn)
	defer func() {
		if success < 2 {
			if success == 1 {
				write(conn, []byte{leaveByte})
			}
			conn.Close()
		}
	}()

	// write header 'HUB0'
	if err := write(conn, []byte("HUB0")); err != nil {
		return nil, err
	}

	// read header 'HUB0'
	header := make([]byte, 4)
	if err := read(conn, header); err != nil {
		return nil, err
	}
	if string(header) != "HUB0" {
		return nil, errors.New("invalid header")
	}
	success++

	// write hub id
	if err := write(conn, hubId.Bytes()); err != nil {
		return nil, err
	}

	// write param
	if err := writemsg(conn, param); err != nil {
		return nil, err
	}

	// read spoke id
	spokeIdBytes := make([]byte, idSize)
	if err := read(conn, spokeIdBytes); err != nil {
		return nil, err
	}
	if !IsValidIdBytes(spokeIdBytes) {
		return nil, errors.New("invalid id")
	}

	success++
	spoke := &TCPSpoke{
		id:   IdBytes(spokeIdBytes),
		conn: conn,
	}
	return spoke, nil
}
