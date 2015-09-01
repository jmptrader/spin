package spin_test

import (
	"encoding/binary"
	"github.com/tidwall/spin"
	"sync"
	"testing"
)

func TestEasy(t *testing.T) {
	const (
		hubCount     = 10
		spokeCount   = 100
		messageCount = 1000
	)
	var wg sync.WaitGroup
	for i := 0; i < hubCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var messages [][]byte
			joined, joinedMax := 0, 0
			hub := spin.NewHub(&spin.Config{
				Join: func(spoke *spin.Spoke, param []byte) ([][]byte, bool) {
					joined++
					joinedMax++
					return messages, true
				},
				Leave: func(spoke *spin.Spoke) {
					joined--
				},
				Feedback: func(spoke *spin.Spoke, data []byte) {

				},
			})
			go func() {
				defer hub.Stop()
				for i := 0; i < messageCount; i++ {
					hub.JoinLock()
					msg := make([]byte, 8)
					binary.LittleEndian.PutUint64(msg, uint64(i))
					messages = append(messages, msg)
					hub.Send(msg)
					hub.JoinUnlock()
				}
			}()
			var wg2 sync.WaitGroup
			for i := 0; i < spokeCount; i++ {
				wg2.Add(1)
				go func() {
					defer wg2.Done()
					spoke := hub.Join(nil)
					defer spoke.Leave()
					spoke.Feedback([]byte{1})
					n := 0
					for {
						msg, ok := spoke.Receive()
						if !ok {
							break
						}
						if len(msg) != 8 || int(binary.LittleEndian.Uint64(msg)) != n {
							t.Fatalf("messages out of order")
						}
						n++
					}
					if n != messageCount {
						t.Fatalf("wrong number of messages sent. %d != %d", n, messageCount)
					}
					//spoke.Feedback([]byte{2})
				}()
			}
			wg2.Wait()
			if joinedMax != spokeCount {
				t.Fatalf("wrong number of spokes joined. %d != %d", joinedMax, spokeCount)
			}
			if joined != 0 {
				t.Fatalf("not all spokes left. %d != 0", joined)
			}
		}()
	}
	wg.Wait()
}
