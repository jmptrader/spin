package spin

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
)

const (
	zeroByte   = 0x00
	nilByte    = 0x01
	leaveByte  = 0x02
	ignoreByte = 0x03
	oneByte    = 0x04
	twoByte    = 0x05
	threeByte  = 0x06
	fourByte   = 0x07
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

// 11111111  11111111  11111111  11111111
// 76543210  76543210  76543210  76543210
// XXXXXXXX  XXXXXXXX  XXXXXXXX  XXXXXNNN
//
func writemsg(conn *net.TCPConn, message []byte) error {
	if message == nil {
		return write(conn, []byte{nilByte})
	}
	if len(message) == 0 {
		return write(conn, []byte{zeroByte})
	}
	var numByte = 0
	if len(message) <= 0x1F {
		numByte = 1
	} else if len(message) <= 0x1FFF {
		numByte = 2
	} else if len(message) <= 0x1FFFFF {
		numByte = 3
	} else if len(message) <= 0x1FFFFFFF {
		numByte = 4
	} else {
		return errors.New("invalid message")
	}
	var val = uint32(len(message)<<3) | uint32(oneByte+numByte-1)
	var b = make([]byte, 4)
	binary.LittleEndian.PutUint32(b, val)
	err := write(conn, b[:numByte])
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
	n := int(bp[0] & 0x07)
	switch n {
	case zeroByte:
		return &msgT{data: []byte{}}, nil
	case nilByte:
		return &msgT{}, nil
	case leaveByte:
		return &msgT{leave: true}, nil
	case ignoreByte:
		return &msgT{ignore: true}, nil
	default:
		n = n - oneByte + 1
	}
	if n > 1 {
		if err := read(conn, bp[1:n]); err != nil {
			return nil, err
		}
	}
	val := int(binary.LittleEndian.Uint32(bp) >> 3)
	data := make([]byte, val)
	if err := read(conn, data); err != nil {
		return nil, err
	}
	return &msgT{data: data}, nil
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
	defer write(conn, []byte{leaveByte})

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
