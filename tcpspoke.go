package spin

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

type TCPSpoke struct {
	id      Id
	conn    *net.TCPConn
	m       *messenger
	writeMu sync.Mutex
	left    bool
}

func (spoke *TCPSpoke) close()                  { spoke.m.close() }
func (spoke *TCPSpoke) send(message []byte)     { spoke.m.send(message) }
func (spoke *TCPSpoke) Receive() ([]byte, bool) { return spoke.m.receive() }
func (spoke *TCPSpoke) Id() Id                  { return spoke.id }
func (spoke *TCPSpoke) Leave() {
	spoke.writeMu.Lock()
	if !spoke.left {
		spoke.conn.Write([]byte{0xFE})
		spoke.conn.Close()
		spoke.close()
		spoke.left = true
	}
	spoke.writeMu.Unlock()
}
func (spoke *TCPSpoke) Feedback(data []byte) {
	spoke.writeMu.Lock()
	writeMessage(spoke.conn, data)
	spoke.writeMu.Unlock()
}

func Dial(addr string, hubId Id, param []byte) (Spoke, error) {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return nil, err
	}
	spokeId, err := func() (spokeId Id, err error) {
		conn.Write([]byte("HUB0"))
		conn.Write(hubId.Bytes())
		if param == nil {
			conn.Write([]byte{0})
		} else {
			err := writeMessage(conn, param)
			if err != nil {
				return spokeId, err
			}
		}
		header := make([]byte, 4+idSize)
		if _, err = io.ReadFull(conn, header); err != nil {
			return spokeId, err
		}
		if header[0] != 'H' || header[1] != 'U' || header[2] != 'B' || header[3] != '0' {
			return spokeId, errors.New("invalid header")
		}
		if !IsValidIdBytes(header[4:]) {
			return spokeId, errors.New("invalid id bytes")
		}
		return IdBytes(header[4:]), nil
	}()
	if err != nil {
		conn.Close()
		return nil, err
	}
	return newTCPSpoke(conn.(*net.TCPConn), spokeId), nil
}

func newTCPSpoke(conn *net.TCPConn, spokeId Id) *TCPSpoke {
	spoke := &TCPSpoke{
		id:   spokeId,
		conn: conn,
		m:    newMessenger(),
	}
	go func() {
		defer conn.Close()
		go func() {
			defer conn.Close()
			for {
				time.Sleep(time.Second)
				spoke.writeMu.Lock()
				_, err := conn.Write([]byte{0xFF})
				spoke.writeMu.Unlock()
				if err != nil {
					return
				}
			}
		}()
		for {
			message, ignore, leave, err := readMessage(conn)
			if err != nil {
				return
			}
			if leave {
				spoke.Leave()
				return
			}
			if !ignore {
				spoke.send(message)
			}
		}
	}()
	return spoke
}
