package spin

import (
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
		return write(conn, []byte{0xFF})
	}
	msgSize := len(message)
	if msgSize == 0 {
		return write(conn, []byte{0x00})
	}
	var err error
	if msgSize <= 0xFF {
		err = write(conn, []byte{0x01, byte(msgSize)})
	} else if msgSize <= 0xFFFF {
		err = write(conn, []byte{0x02, byte(msgSize & 0xFF), byte((msgSize >> 8) & 0xFF)})
	} else if msgSize <= 0xFFFFFF {
		err = write(conn, []byte{0x03, byte(msgSize & 0xFF), byte((msgSize >> 8) & 0xFF), byte((msgSize >> 16) & 0xFF)})
	} else {
		return errors.New("invalid message")
	}
	if err != nil {
		return err
	}
	return write(conn, message)
}

func readmsg(conn *net.TCPConn) (*msgT, error) {
	bp := make([]byte, 4)
	if _, err := conn.Read(bp[:1]); err != nil {
		return nil, err
	}
	n := int(bp[0])
	var msgSize int
	switch n {
	case 0x00:
		return &msgT{}, nil
	case 0xFE:
		return &msgT{leave: true}, nil
	case 0xFF:
		return &msgT{ignore: true}, nil
	case 0x01, 0x02, 0x03:
		if err := read(conn, bp[:n]); err != nil {
			return nil, err
		}
		for i := 0; i < n; i++ {
			msgSize = (msgSize << 8) | int(bp[i])
		}
	default:
		return nil, errors.New("invalid message")
	}
	msg := &msgT{data: make([]byte, msgSize)}
	err := read(conn, msg.data)
	return msg, err
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
	defer write(conn, []byte{0xFE})

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
