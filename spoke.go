// Copyright 2015 Joshua Baker <joshbaker77@gmail.com>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package spin

type Spoke interface {
	Id() Id
	Leave()
	Receive() ([]byte, bool)
	Feedback(data []byte)
}

type LocalSpoke struct {
	id  Id
	hub *Hub
	m   *messengerT
}

func newLocalSpoke(hub *Hub) *LocalSpoke {
	return &LocalSpoke{
		id:  NewId(),
		hub: hub,
		m:   newMessenger(),
	}
}
func (spoke *LocalSpoke) close()                  { spoke.m.close() }
func (spoke *LocalSpoke) send(message []byte)     { spoke.m.send(message) }
func (spoke *LocalSpoke) Receive() ([]byte, bool) { return spoke.m.receive() }
func (spoke *LocalSpoke) Id() Id                  { return spoke.id }
func (spoke *LocalSpoke) Leave()                  { c <- leaveT{spoke.hub, spoke} }
func (spoke *LocalSpoke) Feedback(data []byte)    { c <- feedbackT{spoke.hub, spoke, data} }
