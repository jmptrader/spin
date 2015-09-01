package spin

import (
	"sync"
)

type Spoke struct {
	Id       Id
	hub      *Hub
	messages [][]byte
	closed   bool
	cond     *sync.Cond
}

func newSpoke(hub *Hub) *Spoke {
	return &Spoke{
		Id:   NewId(),
		hub:  hub,
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

func (spoke *Spoke) close() {
	spoke.cond.L.Lock()
	spoke.closed = true
	spoke.cond.Signal()
	spoke.cond.L.Unlock()
}

func (spoke *Spoke) send(message []byte) {
	spoke.cond.L.Lock()
	spoke.messages = append(spoke.messages, message)
	spoke.cond.Signal()
	spoke.cond.L.Unlock()
}

func (spoke *Spoke) Leave() {
	c <- leaveT{spoke.hub, spoke}
}
func (spoke *Spoke) Receive() ([]byte, bool) {
	spoke.cond.L.Lock()
	defer spoke.cond.L.Unlock()
	for {
		if len(spoke.messages) > 0 {
			message := spoke.messages[0]
			spoke.messages = spoke.messages[1:]
			return message, true
		}
		if spoke.closed {
			return nil, false
		}
		spoke.cond.Wait()
	}
	return nil, false
}
func (spoke *Spoke) Feedback(data []byte) {
	c <- feedbackT{spoke.hub, spoke, data}
}
