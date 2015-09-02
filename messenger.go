// Copyright 2015 Joshua Baker <joshbaker77@gmail.com>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package spin

import (
	"sync"
)

type messengerT struct {
	messages [][]byte
	closed   bool
	cond     *sync.Cond
}

func newMessenger() *messengerT {
	return &messengerT{
		cond: sync.NewCond(&sync.Mutex{}),
	}
}
func (m *messengerT) close() {
	m.cond.L.Lock()
	if !m.closed {
		m.closed = true
		m.cond.Broadcast()
	}
	m.cond.L.Unlock()
}
func (m *messengerT) send(message []byte) {
	m.cond.L.Lock()
	if !m.closed {
		m.messages = append(m.messages, message)
		m.cond.Broadcast()
	}
	m.cond.L.Unlock()
}
func (m *messengerT) receive() ([]byte, bool) {
	for {
		m.cond.L.Lock()
		if len(m.messages) > 0 {
			message := m.messages[0]
			m.messages = m.messages[1:]
			m.cond.L.Unlock()
			return message, true
		}
		if m.closed {
			m.cond.L.Unlock()
			return nil, false
		}
		m.cond.Wait()
		m.cond.L.Unlock()
	}
	return nil, false
}
