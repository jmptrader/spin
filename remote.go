package spin

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"
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
			go handleConnection(conn.(*net.TCPConn))
		}
	}()
	return ln, nil
}

func readMessage(r net.Conn) (message []byte, ignore bool, leave bool, err error) {
	b := make([]byte, 1)
	szb := make([]byte, 4)
	var sz int
	_, err = io.ReadFull(r, b)
	if err != nil {
		return nil, false, false, err
	}
	switch b[0] {
	default:
		return nil, false, false, errors.New("invalid size")
	case 0xFF:
		return nil, true, false, nil
	case 0xFE:
		return nil, false, true, nil
	case 0:
		return nil, false, false, nil
	case 1:
		_, err = io.ReadFull(r, szb[:1])
		sz = int(szb[0])
	case 2:
		_, err = io.ReadFull(r, szb[:2])
		sz = int(szb[0]) | int(szb[1])<<8
	case 3:
		_, err = io.ReadFull(r, szb[:3])
		sz = int(szb[0]) | int(szb[1])<<8 | int(szb[2])<<16
	}
	if err != nil {
		return nil, false, false, err
	}
	message = make([]byte, sz)
	_, err = io.ReadFull(r, message)
	if err != nil {
		return nil, false, false, err
	}
	return message, false, false, nil
}

func bytesForSize(size int) []byte {
	if size <= 0xFF {
		return []byte{1, byte((size >> 0) & 0xFF)}
	} else if size <= 0xFFFF {
		return []byte{2, byte((size >> 0) & 0xFF), byte((size >> 8) & 0xFF)}
	} else if size <= 0xFFFFFF {
		return []byte{3, byte((size >> 0) & 0xFF), byte((size >> 8) & 0xFF), byte((size >> 16) & 0xFF)}
	}
	return nil
}

func writeMessage(conn net.Conn, message []byte) error {
	if message == nil {
		_, err := conn.Write([]byte{0})
		return err
	}
	bytes := bytesForSize(len(message))
	if bytes == nil {
		return errors.New("invalid message size")
	}
	_, err := conn.Write(bytes)
	if err != nil {
		return err
	}
	_, err = conn.Write(message)
	if err != nil {
		return err
	}
	return nil
}

func handleConnection(conn *net.TCPConn) {
	defer conn.Close()
	err := func() error {
		header := make([]byte, 4+idSize)
		_, err := io.ReadFull(conn, header)
		if err != nil {
			return err
		}
		if header[0] != 'H' || header[1] != 'U' || header[2] != 'B' || header[3] != '0' {
			return errors.New("invalid header")
		}
		_, err = conn.Write([]byte("HUB0"))
		if err != nil {
			return err
		}
		if !IsValidIdBytes(header[4:]) {
			return errors.New("invalid id bytes")
		}
		id := IdBytes(header[4:])
		t := findT{id, make(chan *Hub)}
		c <- t
		hub := <-t.c
		if hub == nil {
			return errors.New("hub not found")
		}
		param, _, _, err := readMessage(conn)
		if err != nil {
			return err
		}
		spoke := hub.Join(param).(*LocalSpoke)
		var writeMu sync.Mutex
		var smu sync.Mutex
		var sleft bool
		sleave := func() {
			smu.Lock()
			if !sleft {
				writeMu.Lock()
				conn.Write([]byte{0xFE})
				writeMu.Unlock()
				conn.Close()
				spoke.Leave()
				sleft = true
			}
			smu.Unlock()
		}
		defer sleave()

		_, err = conn.Write(spoke.id.Bytes())
		if err != nil {
			return err
		}

		go func() {
			defer sleave()
			for {
				time.Sleep(time.Second)
				writeMu.Lock()
				_, err = conn.Write([]byte{0xFF})
				writeMu.Unlock()
				if err != nil {
					return
				}
			}
		}()

		go func() {
			defer sleave()
			for {
				message, ignore, leave, err := readMessage(conn)
				if err != nil || leave {
					return
				}
				if !ignore {
					spoke.Feedback(message)
				}
			}
		}()

		for {
			message, ok := spoke.Receive()
			if !ok {
				break
			}
			writeMu.Lock()
			err := writeMessage(conn, message)
			writeMu.Unlock()
			if err != nil {
				return err
			}
		}

		return nil
	}()
	if err != nil {
		println("err: " + err.Error())
	}

}
