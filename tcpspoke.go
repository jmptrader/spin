// Copyright 2015 Joshua Baker <joshbaker77@gmail.com>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package spin

import (
	"errors"
	"github.com/gorilla/websocket"
	"io"
	"time"
)

type TCPSpoke struct {
	id   Id
	ws   *websocket.Conn
	msgC chan []byte
	left bool
}

func (spoke *TCPSpoke) Id() Id { return spoke.id }
func (spoke *TCPSpoke) Receive() ([]byte, bool) {
	if spoke.left {
		return nil, false
	}
	for {
		msgt, msg, err := spoke.ws.ReadMessage()
		if err != nil {
			return nil, false
		}
		if msgt == websocket.BinaryMessage {
			return msg, true
		}
	}
}
func (spoke *TCPSpoke) Leave() {
	if spoke.left {
		return
	}
	close(spoke.msgC)
	spoke.ws.Close()
	spoke.left = true
}
func (spoke *TCPSpoke) Feedback(data []byte) {
	if spoke.left {
		return
	}
	spoke.msgC <- data
}
func (spoke *TCPSpoke) Reader() io.Reader {
	return newSpokeReader(spoke)
}

func Dial(addr string, hubId Id, param []byte) (Spoke, error) {
	ws, _, err := websocket.DefaultDialer.Dial("ws://"+addr+Pattern, nil)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			ws.Close()
		}
	}()

	// write hubid
	err = ws.WriteMessage(websocket.BinaryMessage, hubId.Bytes())
	if err != nil {
		return nil, err
	}

	// write param
	if param == nil {
		err = ws.WriteMessage(websocket.TextMessage, []byte{})
	} else {
		err = ws.WriteMessage(websocket.BinaryMessage, param)
	}
	if err != nil {
		return nil, err
	}

	// read spoke id
	msgt, msg, err := ws.ReadMessage()
	if msgt != websocket.BinaryMessage {
		return nil, errors.New("invalid spoke id type")
	}
	if !IsValidIdBytes(msg) {
		return nil, errors.New("invalid id")
	}

	msgC := make(chan []byte)
	go func() {
		t1 := time.NewTicker(time.Second)
		defer t1.Stop()
		for {
			select {
			case <-t1.C:
				ws.WriteMessage(websocket.TextMessage, []byte{})
			case msg, ok := <-msgC:
				if !ok {
					return
				}
				ws.WriteMessage(websocket.BinaryMessage, msg)
			}
		}
	}()
	spoke := &TCPSpoke{
		id:   IdBytes(msg),
		ws:   ws,
		msgC: msgC,
	}
	success = true
	return spoke, nil
}
