package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/events"
	"github.com/gtsteffaniak/go-logger/logger"
)

type messenger struct {
	flusher http.Flusher
	writer  io.Writer
}

// expects message with double quotes around string
func (msgr messenger) sendEvent(eventType, message string) error {
	_, err := fmt.Fprintf(msgr.writer, "data: {\"eventType\":\"%s\",\"message\":%s}\n\n", eventType, message)
	if err != nil {
		return err
	}
	msgr.flusher.Flush()
	return nil
}

func sseHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !(d.user.Permissions.Realtime || d.user.Permissions.Admin) {
		return http.StatusForbidden, fmt.Errorf("realtime is disabled for this user")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sessionId := r.URL.Query().Get("sessionId")
	username := d.user.Username

	f, ok := w.(http.Flusher)
	if !ok {
		logger.Debugf("error: ResponseWriter does not support Flusher. User: %s, SessionId: %s", username, sessionId)
		return http.StatusInternalServerError, fmt.Errorf("streaming not supported")
	}

	msgr := messenger{flusher: f, writer: w}
	clientGone := r.Context().Done()

	// Initial ack
	if err := msgr.sendEvent("acknowledge", "\"connection established\""); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error sending message: %v, user: %s, SessionId: %s", err, username, sessionId)
	}

	// Register this client with the events system
	sendChan := events.Register(username, settings.GetSources(d.user))
	defer events.Unregister(username, sendChan)

	for {
		select {
		case <-d.ctx.Done():
			_ = msgr.sendEvent("notification", "\"the server is shutting down\"")
			return http.StatusOK, nil

		case <-clientGone:
			return http.StatusOK, nil

		case msg := <-events.BroadcastChan:
			if err := msgr.sendEvent(msg.EventType, msg.Message); err != nil {
				return http.StatusInternalServerError, fmt.Errorf("error sending broadcast: %v, user: %s", err, username)
			}

		case msg, ok := <-sendChan:
			if !ok {
				return http.StatusOK, nil
			}
			if err := msgr.sendEvent(msg.EventType, msg.Message); err != nil {
				return http.StatusInternalServerError, fmt.Errorf("error sending targeted message: %v, user: %s", err, username)
			}
		}
	}
}

// OnlyOfficeLogContext stores context for OnlyOffice operations
type OnlyOfficeLogContext struct {
	Username   string
	SessionID  string
	DocumentID string
	FilePath   string
	Source     string
	ShareHash  string
	StartTime  time.Time
}

// OnlyOfficeLogEvent represents a log event for SSE
type OnlyOfficeLogEvent struct {
	EventType  string `json:"eventType"`
	DocumentID string `json:"documentId"`
	Username   string `json:"username"`
	SessionID  string `json:"sessionId"`
	LogLevel   string `json:"logLevel"`
	Message    string `json:"message"`
	Timestamp  string `json:"timestamp"`
	Component  string `json:"component"`
}

// Global map to store OnlyOffice log contexts
var onlyOfficeContexts = make(map[string]*OnlyOfficeLogContext)
var onlyOfficeContextsMutex sync.RWMutex

// Helper functions for OnlyOffice log context management
func createOnlyOfficeLogContext(username, sessionID, documentID, filePath, source, shareHash string) *OnlyOfficeLogContext {
	return &OnlyOfficeLogContext{
		Username:   username,
		SessionID:  sessionID,
		DocumentID: documentID,
		FilePath:   filePath,
		Source:     source,
		ShareHash:  shareHash,
		StartTime:  time.Now(),
	}
}

func storeOnlyOfficeLogContext(documentID string, context *OnlyOfficeLogContext) {
	onlyOfficeContextsMutex.Lock()
	defer onlyOfficeContextsMutex.Unlock()
	onlyOfficeContexts[documentID] = context
}

func getOnlyOfficeLogContext(documentID string) *OnlyOfficeLogContext {
	onlyOfficeContextsMutex.RLock()
	defer onlyOfficeContextsMutex.RUnlock()
	return onlyOfficeContexts[documentID]
}

func removeOnlyOfficeLogContext(documentID string) {
	onlyOfficeContextsMutex.Lock()
	defer onlyOfficeContextsMutex.Unlock()
	delete(onlyOfficeContexts, documentID)
}

func sendOnlyOfficeLogEvent(context *OnlyOfficeLogContext, level, component, message string) {
	if context == nil {
		return
	}

	// Create the log event message
	logData := map[string]interface{}{
		"documentId": context.DocumentID,
		"username":   context.Username,
		"sessionId":  context.SessionID,
		"logLevel":   level,
		"message":    message,
		"timestamp":  time.Now().Format(time.RFC3339),
		"component":  component,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(logData)
	if err != nil {
		logger.Errorf("Failed to marshal OnlyOffice log event: %v", err)
		return
	}

	// Send to specific user only (not broadcast to all users)
	events.SendToUsers("onlyOfficeLog", string(jsonData), []string{context.Username})
}
