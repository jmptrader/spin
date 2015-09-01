package spin

type Config struct {
	Join     func(spoke *Spoke, param []byte) (messages [][]byte, allowed bool)
	Leave    func(spoke *Spoke)
	Feedback func(spoke *Spoke, data []byte)
}

type Hub struct {
	Id     Id
	config *Config
}

func NewHub(config *Config) *Hub {
	hub := &Hub{
		Id:     NewId(),
		config: config,
	}
	newC <- hub
	return hub
}
func (hub *Hub) Stop()               { stopC <- hub }
func (hub *Hub) Send(message []byte) { sendC <- sendT{hub, message} }
func (hub *Hub) JoinLock()           { lockC <- hub }
func (hub *Hub) JoinUnlock()         { unlockC <- hub }
func (hub *Hub) Join(param []byte) *Spoke {
	spoke := &Spoke{
		Id:       NewId(),
		messageC: make(chan []byte, 10),
		hub:      hub,
	}
	joinC <- joinT{hub, spoke, param}
	return spoke
}

type Spoke struct {
	Id       Id
	hub      *Hub
	messageC chan []byte
}

func (spoke *Spoke) Leave() { leaveC <- leaveT{spoke.hub, spoke} }
func (spoke *Spoke) Receive() ([]byte, bool) {
	message, ok := <-spoke.messageC
	return message, ok
}
func (spoke *Spoke) Feedback(data []byte) {
	//feedbackC <- feedbackT{} //spoke.hub, spoke, data}
}
