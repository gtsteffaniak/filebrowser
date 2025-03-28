package events

var BroadcastChan = make(chan EventMessage, 10)

type EventMessage struct {
	EventType string `json:"eventType"`
	Message   string `json:"message"`
}
