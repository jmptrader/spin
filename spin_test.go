package spin_test

import (
	"encoding/binary"
	"fmt"
	"github.com/tidwall/spin"
	"sync"
	"testing"
	"time"
)

const testTimeout = time.Second * 10

func TestBasicRemote(t *testing.T) {
	const addr = ":9090"
	ln, err := spin.Listen(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	left := false
	var feedback [][]byte
	var sparam []byte
	hub := spin.NewHub(&spin.Config{
		Join: func(spoke spin.Spoke, param []byte) ([][]byte, bool) {
			sparam = param
			return nil, true
		},
		Leave: func(spoke spin.Spoke) {
			left = true
		},
		Feedback: func(spoke spin.Spoke, data []byte) {
			feedback = append(feedback, data)
		},
	})
	defer hub.Stop()

	spoke, err := spin.Dial(addr, hub.Id(), []byte("Hello Hub"))
	if err != nil {
		t.Fatal(err)
	}

	hub.Send([]byte("Hello Spoke"))
	message, ok := spoke.Receive()
	if !ok {
		t.Fatal("expecting to receive a message")
	}
	if string(message) != "Hello Spoke" {
		t.Fatal("expecting to receive a message")
	}
	spoke.Feedback([]byte("Feedback 1"))
	spoke.Feedback([]byte("Feedback 2"))
	spoke.Leave()

	// wait a moment for the leave to register
	deadline := time.Now().Add(time.Second)
	for !left && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	if !left {
		t.Fatal("spoke did not leave")
	}
	if len(feedback) != 2 || string(feedback[0]) != "Feedback 1" || string(feedback[1]) != "Feedback 2" {
		t.Fatal("feedback not well received")
	}
	if string(sparam) != "Hello Hub" {
		t.Fatal("expecting 'Hello Hub' for param")
	}
}

func TestLocal(t *testing.T) {
	return
	const (
		hubCount     = 10
		spokeCount   = 100
		messageCount = 1000
	)
	testHubSpinner(t, "", hubCount, spokeCount, messageCount)
}

func TestRemote(t *testing.T) {

	const (
		hubCount     = 5
		spokeCount   = 25
		messageCount = 500
	)
	testHubSpinner(t, ":9191", hubCount, spokeCount, messageCount)
}

func testHubSpinner(t *testing.T, addr string, hubCount, spokeCount, messageCount int) {
	local := addr == ""
	if !local {
		ln, err := spin.Listen(addr)
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
	}
	var wg sync.WaitGroup
	for i := 0; i < hubCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var messages [][]byte
			feedback, joined, left := 0, 0, 0
			hub := spin.NewHub(&spin.Config{
				Join: func(spoke spin.Spoke, param []byte) ([][]byte, bool) {
					joined++
					return messages, true
				},
				Leave: func(spoke spin.Spoke) {
					left++
				},
				Feedback: func(spoke spin.Spoke, data []byte) {
					feedback++
				},
			})
			hubId := hub.Id()
			var wg3 sync.WaitGroup
			wg3.Add(1)
			go func() {
				defer wg3.Done()
				// wait for a spokes to leave
				defer func() {
					deadline := time.Now().Add(testTimeout)
					for left != spokeCount && time.Now().Before(deadline) {
						time.Sleep(time.Millisecond * 10)
					}
				}()
				defer hub.Stop()
				// wait for all feedback
				defer func() {
					deadline := time.Now().Add(testTimeout)
					for feedback != spokeCount*2 && time.Now().Before(deadline) {
						time.Sleep(time.Millisecond * 10)
					}
				}()
				// wait for all spokes to joined
				defer func() {
					deadline := time.Now().Add(testTimeout)
					for joined != spokeCount && time.Now().Before(deadline) {
						time.Sleep(time.Millisecond * 10)
					}
				}()
				for i := 0; i < messageCount; i++ {
					hub.JoinLock()
					var msg []byte
					if i%800 == 0 {
						msg = make([]byte, 0xFFFF)
					} else {
						msg = make([]byte, 4)
					}
					binary.LittleEndian.PutUint32(msg[:4], uint32(i))
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
					var err error
					param := make([]byte, 8)
					binary.LittleEndian.PutUint64(param, uint64(i))
					var spoke spin.Spoke
					if local {
						spoke = hub.Join(param)
					} else {
						spoke, err = spin.Dial(addr, hubId, param)
						if err != nil {
							t.Fatal(err)
						}
					}
					defer spoke.Leave()
					spoke.Feedback([]byte{1})
					n := 0
					for {
						msg, ok := spoke.Receive()
						if !ok {
							break
						}
						if len(msg) < 4 || int(binary.LittleEndian.Uint32(msg[:4])) != n {
							t.Fatalf("messages out of order")
						}
						n++
						if n == 1 {
							spoke.Feedback([]byte{2})
						}
					}
					if n != messageCount {
						t.Fatalf("wrong number of messages sent. %d != %d", n, messageCount)
					}

				}()
			}
			wg3.Wait()
			wg2.Wait()
			if joined != spokeCount {
				t.Fatalf("not all spokes joined. %d != %d", joined, spokeCount)
			}
			if left != spokeCount {
				t.Fatalf("not all spokes left. %d != %d", left, spokeCount)
			}
			if feedback != spokeCount*2 {
				t.Fatalf("not all feedback sent. %d != %d", feedback, spokeCount*2)
			}
		}()
	}
	wg.Wait()
	fmt.Printf("sent %d messages\n", hubCount*spokeCount*messageCount)
}
