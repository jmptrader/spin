package spin

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
)

// Protocol
// ------------------------------------------
// Client Header = 'HUB0'+HubId
// Server Header = 'HUB0'+SpokeId
// HubId,SpokeId = 16-byte Id
// Messages = Type+SizeN+Size+Payload

type Listener struct {
	tcp net.Listener
}

func (ln *Listener) Addr() net.Addr {
	return ln.tcp.Addr()
}

func (ln *Listener) Close() error {
	return ln.tcp.Close()
}

func Listen(addr string) (*Listener, error) {
	tcp, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	ln := &Listener{tcp}
	go func() {
		for {
			conn, err := ln.tcp.Accept()
			if err != nil {
				return
			}
			go handle(conn.(*net.TCPConn))
		}
	}()
	return ln, nil
}

func read(conn *net.TCPConn, p []byte) error {
	_, err := io.ReadFull(conn, p)
	return err
}
func write(conn *net.TCPConn, p []byte) error {
	_, err := conn.Write(p)
	return err
}

type msgT struct {
	ignore bool
	leave  bool
	data   []byte
}

func writemsg(conn *net.TCPConn, message []byte) error {
	if message == nil {
		return write(conn, []byte{0xFF, 0x00, 0x00, 0x00, 0x00})
	}
	b := make([]byte, 5)
	b[0] = 0x04
	binary.LittleEndian.PutUint32(b[1:], uint32(len(message)))
	err := write(conn, b)
	if err != nil {
		return err
	}
	return write(conn, message)
}

func readmsg(conn *net.TCPConn) (*msgT, error) {
	bp := make([]byte, 5)
	if err := read(conn, bp); err != nil {
		return nil, err
	}
	switch bp[0] {
	case 0x00:
		return &msgT{}, nil
	case 0xFE:
		return &msgT{leave: true}, nil
	case 0xFF:
		return &msgT{ignore: true}, nil
	case 0x04:
		n := int(binary.LittleEndian.Uint32(bp[1:]))
		msg := &msgT{data: make([]byte, n)}
		err := read(conn, msg.data)
		if err != nil {
			return nil, err
		}
		return msg, nil
	}
	return nil, errors.New("invalid message")
}

func handle(conn *net.TCPConn) {
	defer conn.Close()

	// read header 'HUB0'
	header := make([]byte, 4)
	if err := read(conn, header); err != nil {
		return
	}
	if string(header) != "HUB0" {
		return
	}
	defer write(conn, []byte{0xFE, 0x00, 0x00, 0x00, 0x00})

	// write header 'HUB0'
	if err := write(conn, []byte("HUB0")); err != nil {
		return
	}

	// read hub id
	hubIdBytes := make([]byte, idSize)
	if err := read(conn, hubIdBytes); err != nil {
		return
	}
	if !IsValidIdBytes(hubIdBytes) {
		return
	}
	hubId := IdBytes(hubIdBytes)

	// read param
	param, err := readmsg(conn)
	if err != nil {
		return
	}

	// find hub
	t := findT{hubId, make(chan *Hub)}
	c <- t
	hub := <-t.c
	if hub == nil {
		return
	}

	// join
	spoke := hub.Join(param.data)
	defer spoke.Leave()

	// write spoke id
	err = write(conn, spoke.Id().Bytes())
	if err != nil {
		return
	}

	// feedback reader
	go func() {
		defer spoke.Leave()
		for {
			msg, err := readmsg(conn)
			if err != nil || msg.leave {
				return
			}
			if !msg.ignore {
				spoke.Feedback(msg.data)
			}
		}
	}()

	// message writer
	for {
		message, ok := spoke.Receive()
		if !ok {
			return
		}
		err = writemsg(conn, message)
		if err != nil {
			return
		}
	}

}
