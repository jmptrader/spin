package spin

type joinT struct {
	hub   *Hub
	spoke *Spoke
	param []byte
}
type leaveT struct {
	hub   *Hub
	spoke *Spoke
}
type sendT struct {
	hub     *Hub
	message []byte
}
type feedbackT struct {
	hub   *Hub
	spoke *Spoke
	data  []byte
}

type lockedT struct {
	param    []byte
	feedback [][]byte
}

var newC = make(chan *Hub)
var stopC = make(chan *Hub)
var lockC = make(chan *Hub)
var unlockC = make(chan *Hub)
var joinC = make(chan joinT)
var leaveC = make(chan leaveT)
var sendC = make(chan sendT)
var feedbackC = make(chan feedbackT)

type spokesT struct {
	locked   map[*Spoke]*lockedT
	unlocked map[*Spoke]bool
}

func init() {
	go func() {
		var hubs = make(map[*Hub]*spokesT)
		for {
			select {
			case hub := <-newC:
				hubs[hub] = &spokesT{
					locked:   nil,
					unlocked: make(map[*Spoke]bool),
				}
			case hub := <-stopC:
				if spokes, ok := hubs[hub]; ok {
					if spokes.locked != nil {
						for spoke, _ := range spokes.locked {
							close(spoke.messageC)
						}
					}
					for spoke, _ := range spokes.unlocked {
						close(spoke.messageC)
						if hub.config != nil && hub.config.Leave != nil {
							hub.config.Leave(spoke)
						}
					}
				}
				delete(hubs, hub)
			case send := <-sendC:
				if spokes, ok := hubs[send.hub]; ok {
					for spoke, _ := range spokes.unlocked {
						spoke.messageC <- send.message
					}
				}
			case feedback := <-feedbackC:
				feedback = feedback
				// if spokes, ok := hubs[feedback.hub]; ok {
				// 	if _, ok := spokes.unlocked[feedback.spoke]; ok {
				// 		if feedback.hub.config != nil && feedback.hub.config.Feedback != nil {
				// 			feedback.hub.config.Feedback(feedback.spoke, feedback.data)
				// 		}
				// 	} else if spokes.locked != nil {
				// 		if locked, ok := spokes.locked[feedback.spoke]; ok {
				// 			locked.feedback = append(locked.feedback, feedback.data)
				// 		}
				// 	}

				// }
			case hub := <-lockC:
				if spokes, ok := hubs[hub]; ok {
					if spokes.locked != nil {
						panic("already locked")
					}
					spokes.locked = make(map[*Spoke]*lockedT)
				}
			case hub := <-unlockC:
				if spokes, ok := hubs[hub]; ok {
					if spokes.locked == nil {
						panic("not locked")
					}
					for spoke, locked := range spokes.locked {
						var messages [][]byte
						var allowed bool
						if hub.config != nil && hub.config.Join != nil {
							messages, allowed = hub.config.Join(spoke, locked.param)
						}
						if allowed {
							spokes.unlocked[spoke] = true
							for _, message := range messages {
								spoke.messageC <- message
							}
							// if hub.config != nil && hub.config.Feedback != nil {
							// 	for _, feedback := range locked.feedback {
							// 		hub.config.Feedback(spoke, feedback)
							// 	}
							// }
						} else {
							close(spoke.messageC)
						}
					}
					spokes.locked = nil
				}
			case join := <-joinC:
				if spokes, ok := hubs[join.hub]; ok {
					if spokes.locked != nil {
						spokes.locked[join.spoke] = &lockedT{join.param, nil}
					} else {
						var messages [][]byte
						var allowed bool
						if join.hub.config != nil && join.hub.config.Join != nil {
							messages, allowed = join.hub.config.Join(join.spoke, join.param)
						}
						if allowed {
							spokes.unlocked[join.spoke] = true
							for _, message := range messages {
								join.spoke.messageC <- message
							}
						} else {
							close(join.spoke.messageC)
						}
					}
				} else {
					close(join.spoke.messageC)
				}
			case leave := <-leaveC:
				if spokes, ok := hubs[leave.hub]; ok {
					if _, ok := spokes.unlocked[leave.spoke]; ok {
						delete(spokes.unlocked, leave.spoke)
						close(leave.spoke.messageC)
						if leave.hub.config != nil && leave.hub.config.Leave != nil {
							leave.hub.config.Leave(leave.spoke)
						}
					} else if spokes.locked != nil {
						if _, ok := spokes.locked[leave.spoke]; ok {
							delete(spokes.locked, leave.spoke)
							close(leave.spoke.messageC)
						}
					}
				}
			}
		}
	}()
}
