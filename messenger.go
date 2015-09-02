package spin

import (
	"sync"
)

type messenger struct {
	messages [][]byte
	closed   bool
	cond     *sync.Cond
}

func newMessenger() *messenger {
	return &messenger{
		cond: sync.NewCond(&sync.Mutex{}),
	}
}
func (m *messenger) close() {
	m.cond.L.Lock()
	m.closed = true
	m.cond.Signal()
	m.cond.L.Unlock()
}
func (m *messenger) send(message []byte) {
	m.cond.L.Lock()
	m.messages = append(m.messages, message)
	m.cond.Signal()
	m.cond.L.Unlock()
}
func (m *messenger) receive() ([]byte, bool) {
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
