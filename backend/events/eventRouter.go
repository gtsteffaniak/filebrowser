package events

import (
	"sync"
)

type EventMessage struct {
	EventType string `json:"eventType"`
	Message   string `json:"message"`
}

type sourceEvent struct {
	source string
	event  EventMessage
}

type userEvent struct {
	event EventMessage
	users []string
}

var (
	BroadcastChan     = make(chan EventMessage, 100)
	userEventChan     = make(chan userEvent, 100)
	debounceInputChan = make(chan EventMessage, 100)
	sourceUpdateChan  = make(chan sourceEvent, 100)

	userClientsMu   sync.RWMutex
	userClients     = make(map[string][]chan EventMessage)
	sourceClientsMu sync.RWMutex
	sourceClients   = make(map[string]map[chan EventMessage]struct{})
)

func init() {
	go handleUserEvents()
	go handleSourceUpdates() // Add this

}

func handleUserEvents() {
	for ue := range userEventChan {
		for _, user := range ue.users {
			userClientsMu.RLock()
			conns := userClients[user]
			userClientsMu.RUnlock()

			for _, ch := range conns {
				select {
				case ch <- ue.event:
				default:
				}
			}
		}
	}
}

func Register(username string, sources []string) chan EventMessage {
	ch := make(chan EventMessage, 10)

	userClientsMu.Lock()
	userClients[username] = append(userClients[username], ch)
	userClientsMu.Unlock()

	sourceClientsMu.Lock()
	for _, source := range sources {
		if sourceClients[source] == nil {
			sourceClients[source] = make(map[chan EventMessage]struct{})
		}
		sourceClients[source][ch] = struct{}{}
	}
	sourceClientsMu.Unlock()

	return ch
}

func Unregister(username string, ch chan EventMessage) {
	userClientsMu.Lock()
	defer userClientsMu.Unlock()
	conns, ok := userClients[username]
	if !ok {
		// User already cleaned up by Shutdown, nothing to do.
		return
	}
	for i, c := range conns {
		if c == ch {
			userClients[username] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
	if len(userClients[username]) == 0 {
		delete(userClients, username)
	}

	sourceClientsMu.Lock()
	defer sourceClientsMu.Unlock()
	for source, clients := range sourceClients {
		delete(clients, ch)
		if len(clients) == 0 {
			delete(sourceClients, source)
		}
	}
}

func SendToUsers(eventType, message string, users []string) {
	userEventChan <- userEvent{
		event: EventMessage{EventType: eventType, Message: message},
		users: users,
	}
}

func SendSourceUpdate(source string, message string) {
	sourceUpdateChan <- sourceEvent{
		source: source,
		event: EventMessage{
			EventType: "sourceUpdate",
			Message:   message,
		},
	}
}

func DebouncedBroadcast(eventType, message string) {
	debounceInputChan <- EventMessage{
		EventType: eventType,
		Message:   message,
	}
}

func handleSourceUpdates() {
	for update := range sourceUpdateChan {
		sourceClientsMu.RLock()
		clients := sourceClients[update.source]
		sourceClientsMu.RUnlock()

		for ch := range clients {
			select {
			case ch <- update.event:
			default:
				// Optional: log dropped message
			}
		}
	}
}

func Shutdown() {
	userClientsMu.Lock()
	defer userClientsMu.Unlock()
	sourceClientsMu.Lock()
	defer sourceClientsMu.Unlock()

	for username, clientChannels := range userClients {
		for _, ch := range clientChannels {
			// Clean up source clients
			for source, clients := range sourceClients {
				delete(clients, ch)
				if len(clients) == 0 {
					delete(sourceClients, source)
				}
			}
			close(ch)
		}
		delete(userClients, username)
	}
}
