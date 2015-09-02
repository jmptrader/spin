// Copyright 2015 Joshua Baker <joshbaker77@gmail.com>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package spin

type Config struct {
	Join     func(spoke Spoke, param []byte) (messages [][]byte, allowed bool)
	Leave    func(spoke Spoke)
	Feedback func(spoke Spoke, data []byte)
}
type Hub struct {
	id     Id
	config *Config
}

func NewHub(config *Config) *Hub {
	hub := &Hub{
		id:     NewId(),
		config: config,
	}
	c <- newT{hub}
	return hub
}
func (hub *Hub) Id() Id              { return hub.id }
func (hub *Hub) Stop()               { c <- stopT{hub} }
func (hub *Hub) Send(message []byte) { c <- sendT{hub, message} }
func (hub *Hub) JoinLock()           { c <- lockT{hub} }
func (hub *Hub) JoinUnlock()         { c <- unlockT{hub} }
func (hub *Hub) Join(param []byte) Spoke {
	spoke := newLocalSpoke(hub)
	c <- joinT{hub, spoke, param}
	return spoke
}

func FindHub(hubId Id) *Hub {
	t := findT{hubId, make(chan *Hub)}
	c <- t
	return <-t.c
}
