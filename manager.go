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
type newT struct{ hub *Hub }
type stopT struct{ hub *Hub }
type lockT struct{ hub *Hub }
type unlockT struct{ hub *Hub }

var c = make(chan interface{})

type spokesT struct {
	locked   map[*Spoke]*lockedT
	unlocked map[*Spoke]bool
}

func init() {
	go func() {
		var hubs = make(map[*Hub]*spokesT)
		for c := range c {
			switch c := c.(type) {
			case newT:
				hubs[c.hub] = &spokesT{
					locked:   nil,
					unlocked: make(map[*Spoke]bool),
				}
			case stopT:
				if spokes, ok := hubs[c.hub]; ok {
					if spokes.locked != nil {
						for spoke, _ := range spokes.locked {
							spoke.close()
						}
					}
					for spoke, _ := range spokes.unlocked {
						spoke.close()
						if c.hub.config != nil && c.hub.config.Leave != nil {
							c.hub.config.Leave(spoke)
						}
					}
				}
				delete(hubs, c.hub)
			case sendT:
				if spokes, ok := hubs[c.hub]; ok {
					for spoke, _ := range spokes.unlocked {
						spoke.send(c.message)
					}
				}
			case lockT:
				if spokes, ok := hubs[c.hub]; ok {
					if spokes.locked != nil {
						panic("already locked")
					}
					spokes.locked = make(map[*Spoke]*lockedT)
				}
			case unlockT:
				if spokes, ok := hubs[c.hub]; ok {
					if spokes.locked == nil {
						panic("not locked")
					}
					for spoke, locked := range spokes.locked {
						var messages [][]byte
						var allowed bool
						if c.hub.config != nil && c.hub.config.Join != nil {
							messages, allowed = c.hub.config.Join(spoke, locked.param)
						}
						if allowed {
							spokes.unlocked[spoke] = true
							for _, message := range messages {
								spoke.send(message)
							}
							if c.hub.config != nil && c.hub.config.Feedback != nil {
								for _, feedback := range locked.feedback {
									c.hub.config.Feedback(spoke, feedback)
								}
							}
						} else {
							spoke.close()
						}
					}
					spokes.locked = nil
				}
			case feedbackT:
				if spokes, ok := hubs[c.hub]; ok {
					if _, ok := spokes.unlocked[c.spoke]; ok {
						if c.hub.config != nil && c.hub.config.Feedback != nil {
							c.hub.config.Feedback(c.spoke, c.data)
						}
					} else if spokes.locked != nil {
						if locked, ok := spokes.locked[c.spoke]; ok {
							locked.feedback = append(locked.feedback, c.data)
						}
					}
				}
			case joinT:
				if spokes, ok := hubs[c.hub]; ok {
					if spokes.locked != nil {
						spokes.locked[c.spoke] = &lockedT{c.param, nil}
					} else {
						var messages [][]byte
						var allowed bool
						if c.hub.config != nil && c.hub.config.Join != nil {
							messages, allowed = c.hub.config.Join(c.spoke, c.param)
						}
						if allowed {
							spokes.unlocked[c.spoke] = true
							for _, message := range messages {
								c.spoke.send(message)
							}
						} else {
							c.spoke.close()
						}
					}
				} else {
					c.spoke.close()
				}
			case leaveT:
				if spokes, ok := hubs[c.hub]; ok {
					if _, ok := spokes.unlocked[c.spoke]; ok {
						delete(spokes.unlocked, c.spoke)
						c.spoke.close()
						if c.hub.config != nil && c.hub.config.Leave != nil {
							c.hub.config.Leave(c.spoke)
						}
					} else if spokes.locked != nil {
						if _, ok := spokes.locked[c.spoke]; ok {
							delete(spokes.locked, c.spoke)
							c.spoke.close()
						}
					}
				}
			}
		}
	}()
}
