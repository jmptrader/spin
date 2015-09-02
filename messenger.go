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
	m.closed = true
	m.cond.Signal()
	m.cond.L.Unlock()
}
func (m *messengerT) send(message []byte) {
	m.cond.L.Lock()
	m.messages = append(m.messages, message)
	m.cond.Signal()
	m.cond.L.Unlock()
}
func (m *messengerT) receive() ([]byte, bool) {
	m.cond.L.Lock()
	defer m.cond.L.Unlock()
	for {
		if len(m.messages) > 0 {
			message := m.messages[0]
			m.messages = m.messages[1:]
			return message, true
		}
		if m.closed {
			return nil, false
		}
		m.cond.Wait()
	}
	return nil, false
}
