package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/events"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Messenger streams Server-Sent Events to a client.
type Messenger struct {
	flusher http.Flusher
	writer  io.Writer
}

// NewMessenger creates an SSE messenger for the given writer/flusher pair.
func NewMessenger(flusher http.Flusher, writer io.Writer) Messenger {
	return Messenger{flusher: flusher, writer: writer}
}

// SendEvent writes one SSE data frame.
func (msgr Messenger) SendEvent(eventType, message string) error {
	_, err := fmt.Fprintf(msgr.writer, "data: {\"eventType\":\"%s\",\"message\":%s}\n\n", eventType, message)
	if err != nil {
		return err
	}
	msgr.flusher.Flush()
	return nil
}

// SSEHandler streams server events to authenticated users with realtime permission.
func SSEHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if !(d.User.Permissions.Realtime || d.User.Permissions.Admin) {
		return http.StatusForbidden, fmt.Errorf("realtime is disabled for this user")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sessionId := r.URL.Query().Get("sessionId")
	username := d.User.Username

	f, ok := w.(http.Flusher)
	if !ok {
		logger.Debugf("error: ResponseWriter does not support Flusher. User: %s, SessionId: %s", username, sessionId)
		return http.StatusInternalServerError, fmt.Errorf("streaming not supported")
	}

	msgr := NewMessenger(f, w)
	clientGone := r.Context().Done()

	if err := msgr.SendEvent("acknowledge", "\"connection established\""); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error sending message: %v, user: %s, SessionId: %s", err, username, sessionId)
	}

	sendChan := events.Register(username, d.User.GetSourceNames())
	defer events.Unregister(username, sendChan)

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-d.Ctx.Done():
			_ = msgr.SendEvent("notification", "\"the server is shutting down\"")
			return http.StatusOK, nil

		case <-clientGone:
			return http.StatusOK, nil

		case <-heartbeatTicker.C:
			if err := msgr.SendEvent("heartbeat", "\"hb\""); err != nil {
				return http.StatusInternalServerError, fmt.Errorf("error sending heartbeat: %v, user: %s", err, username)
			}

		case msg := <-events.BroadcastChan:
			if err := msgr.SendEvent(msg.EventType, msg.Message); err != nil {
				return http.StatusInternalServerError, fmt.Errorf("error sending broadcast: %v, user: %s", err, username)
			}

		case msg, ok := <-sendChan:
			if !ok {
				return http.StatusOK, nil
			}
			if err := msgr.SendEvent(msg.EventType, msg.Message); err != nil {
				return http.StatusInternalServerError, fmt.Errorf("error sending targeted message: %v, user: %s", err, username)
			}
		}
	}
}

// OnlyOfficeLogContext stores context for OnlyOffice operations.
type OnlyOfficeLogContext struct {
	Username   string
	SessionID  string
	DocumentID string
	FilePath   string
	Source     string
	ShareHash  string
	isAdmin    bool
	StartTime  time.Time
}

var (
	onlyOfficeContexts      = make(map[string]*OnlyOfficeLogContext)
	onlyOfficeContextsMutex sync.RWMutex
)

// CreateOnlyOfficeLogContext stores metadata for an OnlyOffice editing session.
func CreateOnlyOfficeLogContext(username, sessionID, documentID, filePath, source, shareHash string, isAdmin bool) *OnlyOfficeLogContext {
	return &OnlyOfficeLogContext{
		Username:   username,
		SessionID:  sessionID,
		DocumentID: documentID,
		FilePath:   filePath,
		Source:     source,
		ShareHash:  shareHash,
		StartTime:  time.Now(),
		isAdmin:    isAdmin,
	}
}

// StoreOnlyOfficeLogContext registers a log context by document ID.
func StoreOnlyOfficeLogContext(documentID string, context *OnlyOfficeLogContext) {
	onlyOfficeContextsMutex.Lock()
	defer onlyOfficeContextsMutex.Unlock()
	onlyOfficeContexts[documentID] = context
}

// GetOnlyOfficeLogContext returns a stored log context.
func GetOnlyOfficeLogContext(documentID string) *OnlyOfficeLogContext {
	onlyOfficeContextsMutex.RLock()
	defer onlyOfficeContextsMutex.RUnlock()
	return onlyOfficeContexts[documentID]
}

// RemoveOnlyOfficeLogContext drops a stored log context.
func RemoveOnlyOfficeLogContext(documentID string) {
	onlyOfficeContextsMutex.Lock()
	defer onlyOfficeContextsMutex.Unlock()
	delete(onlyOfficeContexts, documentID)
}

// SendOnlyOfficeLogEvent emits an OnlyOffice log event over SSE.
func SendOnlyOfficeLogEvent(context *OnlyOfficeLogContext, level, component, message string) {
	if context == nil || !context.isAdmin {
		return
	}

	logData := map[string]interface{}{
		"documentId": context.DocumentID,
		"username":   context.Username,
		"sessionId":  context.SessionID,
		"logLevel":   level,
		"message":    message,
		"timestamp":  time.Now().Format(time.RFC3339),
		"component":  component,
	}

	jsonData, err := json.Marshal(logData)
	if err != nil {
		logger.Errorf("Failed to marshal OnlyOffice log event: %v", err)
		return
	}

	events.BroadcastChan <- events.EventMessage{
		EventType: "onlyOfficeLog",
		Message:   string(jsonData),
	}
}
