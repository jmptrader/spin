// Copyright 2015 Joshua Baker <joshbaker77@gmail.com>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package spin

import (
	"errors"
	"github.com/gorilla/websocket"
	"net/http"
	"time"
)

const Pattern = "/595487fd/47166a1d/1178f6b5/1abae790/d16e8937"

func Handler(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		return
	}
	defer ws.Close()
	err = socket(ws)
	if err != nil {
		return
	}
}

func handshake(ws *websocket.Conn) (Spoke, error) {
	// read hub id
	msgt, msg, err := ws.ReadMessage()
	if err != nil {
		return nil, err
	}
	if msgt != websocket.BinaryMessage {
		return nil, errors.New("invalid message type for hub id")
	}
	if !IsValidIdBytes(msg) {
		return nil, errors.New("invalid hub id bytes")
	}
	hubId := IdBytes(msg)

	// read param
	var param []byte
	msgt, msg, err = ws.ReadMessage()
	if err != nil {
		return nil, err
	}
	if msgt == websocket.BinaryMessage {
		param = msg
	}

	// find hub
	hub := FindHub(hubId)
	if hub == nil {
		return nil, errors.New("hub not found")
	}

	// join
	spoke := hub.Join(param)
	err = ws.WriteMessage(websocket.BinaryMessage, spoke.Id().Bytes())
	if err != nil {
		spoke.Leave()
		return nil, err
	}
	return spoke, nil
}

func socket(ws *websocket.Conn) error {
	spoke, err := handshake(ws)
	if err != nil {
		return err
	}
	defer spoke.Leave()

	msgC := make(chan []byte)
	go func() {
		defer func() {
			close(msgC)
		}()
		for {
			msg, ok := spoke.Receive()
			if !ok {
				return
			}
			msgC <- msg
		}
	}()
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
	for {
		msgt, msg, err := ws.ReadMessage()
		if err != nil {
			return err
		}
		if msgt == websocket.BinaryMessage {
			spoke.Feedback(msg)
		}
	}
}
