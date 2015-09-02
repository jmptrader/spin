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
// Type = 1-bit, 0 = keepalive, 1 = message
// SizeN = 7-bits, Number of bytes in Size, 0-3
// Size = Payload Size

var mu sync.Mutex
var ln net.Listener

func Listen(addr string) error {
	mu.Lock()
	defer mu.Unlock()
	if ln != nil {
		return errors.New("already listening")
	}
	var err error
	ln, err = net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {

			}
			go handleConnection(conn.(*net.TCPConn))
		}
	}()
	return nil
}

func byteForSize(size int) []byte {
	if size <= 0xFF {
		return []byte{1, byte((size >> 0) & 0xFF)}
	} else if size <= 0xFFFF {
		return []byte{2, byte((size >> 0) & 0xFF), byte((size >> 8) & 0xFF)}
	} else if size <= 0xFFFFFF {
		return []byte{3, byte((size >> 0) & 0xFF), byte((size >> 8) & 0xFF), byte((size >> 16) & 0xFF)}
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
		spoke := newSpoke(hub)
		defer spoke.Leave()
		_, err = conn.Write(spoke.Id.Bytes())
		if err != nil {
			return err
		}

		var writeMu sync.Mutex
		go func() {
			defer spoke.close()
			for {
				time.Sleep(time.Second)
				writeMu.Lock()
				_, err = conn.Write([]byte{0})
				writeMu.Unlock()
				if err != nil {
					return
				}
			}
		}()

		go func() {
			defer spoke.close()
			b := make([]byte, 1)
			szb := make([]byte, 4)
			for {
				var sz int
				_, err := io.ReadFull(conn, b)
				switch b[0] {
				default:
					return
				case 0:
					continue
				case 1:
					_, err = io.ReadFull(conn, szb[:1])
					sz = int(szb[0])
				case 2:
					_, err = io.ReadFull(conn, szb[:2])
					sz = int(szb[0]) | int(szb[1])<<8
				case 3:
					_, err = io.ReadFull(conn, szb[:3])
					sz = int(szb[0]) | int(szb[1])<<8 | int(szb[2])<<16
				}
				if err != nil {
					return // error
				}
				message := make([]byte, sz)
				_, err = io.ReadFull(conn, message)
				if err != nil {
					return // error
				}
				spoke.Feedback(message)
			}
		}()

		for {
			message, ok := spoke.Receive()
			if !ok {
				break
			}
			bytes := byteForSize(len(message))
			if bytes == nil {
				return errors.New("invalid message size")
			}
			writeMu.Lock()
			_, err = conn.Write(bytes)
			if err != nil {
				return err
			}
			_, err = conn.Write(message)
			if err != nil {
				return err
			}
			writeMu.Unlock()
		}

		return nil
	}()
	if err != nil {
		println("err: " + err.Error())
	}

}
