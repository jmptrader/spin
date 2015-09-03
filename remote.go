// Copyright 2015 Joshua Baker <joshbaker77@gmail.com>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package spin

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
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

const Pattern = "/595487fd/47166a1d/1178f6b5/1abae790/d16e8937"

var httpHead = []byte("GET " + Pattern + " HTTP/1.1\r\n\r\n")

// Protocol
// --------------------------------------------------------
// Client Sends Header = 'HUB0'
// Server Sends Header = 'HUB0'
// Client Sends HubId = 16bytes
// Client Sends Param Message
// Server Sends SpokeId = 16bytes
// Server <--> Client Messages
// Message = Type+Size+Payload
// Type = 3bits. Remaining 5bits belong to Size.
// Size = 5-29bits determined by the Type.
// Payload = Raw Bytes

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

func read(r io.Reader, p []byte) error {
	_, err := io.ReadFull(r, p)
	return err
}
func write(w io.Writer, p []byte) error {
	_, err := w.Write(p)
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
func writemsg(w io.Writer, message []byte) error {
	if message == nil {
		return write(w, []byte{nilByte})
	}
	if len(message) == 0 {
		return write(w, []byte{zeroByte})
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
	err := write(w, b[:numByte])
	if err != nil {
		return err
	}
	return write(w, message)
}

func readmsg(r io.Reader) (*msgT, error) {
	bp := make([]byte, 4)
	if _, err := r.Read(bp[:1]); err != nil {
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
		if err := read(r, bp[1:n]); err != nil {
			return nil, err
		}
	}
	val := int(binary.LittleEndian.Uint32(bp) >> 3)
	data := make([]byte, val)
	if err := read(r, data); err != nil {
		return nil, err
	}
	return &msgT{data: data}, nil
}

func Handler(w http.ResponseWriter, r *http.Request) {
	if hj, ok := w.(http.Hijacker); ok {
		conn, b, err := hj.Hijack()
		if err != nil {
			return
		}
		Hijack(conn, b)
	}
}

func Hijack(conn net.Conn, b *bufio.ReadWriter) {
	defer conn.Close()

	// read header 'HUB0'
	header := make([]byte, 4)
	if err := read(b, header); err != nil {
		return
	}
	if string(header) != "HUB0" {
		return
	}
	defer func() {
		write(b, []byte{leaveByte})
		b.Flush()
	}()

	// write header 'HUB0'
	if err := write(b, []byte("HUB0")); err != nil {
		return
	}
	b.Flush()

	// read hub id
	hubIdBytes := make([]byte, idSize)
	if err := read(b, hubIdBytes); err != nil {
		return
	}
	if !IsValidIdBytes(hubIdBytes) {
		return
	}
	hubId := IdBytes(hubIdBytes)

	// read param
	param, err := readmsg(b)
	if err != nil {
		return
	}

	hub := FindHub(hubId)
	if hub == nil {
		return
	}

	// join
	spoke := hub.Join(param.data)
	defer spoke.Leave()

	// write spoke id
	err = write(b, spoke.Id().Bytes())
	if err != nil {
		return
	}
	b.Flush()

	// feedback reader
	go func() {
		defer spoke.Leave()
		for {
			msg, err := readmsg(b)
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
		err = writemsg(b, message)
		if err != nil {
			return
		}
		b.Flush()
	}
}

func handle(conn net.Conn) {
	b := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	// read special http head line
	hhead := make([]byte, len(httpHead))
	if err := read(b, hhead); err != nil {
		conn.Close()
		return
	}
	if !bytes.Equal(httpHead, hhead) {
		conn.Close()
		return
	}
	Hijack(conn, b)
}
