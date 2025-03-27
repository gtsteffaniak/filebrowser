package events

var BroadcastChan = make(chan EventMessage)

type EventMessage struct {
	EventType string `json:"eventType"`
	Message   string `json:"message"`
}
