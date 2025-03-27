package http

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/events"
)

type messenger struct {
	flusher http.Flusher
	writer  io.Writer
}

func (msgr messenger) sendEvent(eventType, message string) error {
	_, err := fmt.Fprintf(msgr.writer, "data: {\"eventType\":\"%s\",\"message\":\"%s\"}\n\n", eventType, message)
	if err != nil {
		return err
	}
	msgr.flusher.Flush() // Flush to send immediately
	return nil
}

// Handle SSE connection
func sseHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Realtime {
		return http.StatusForbidden, fmt.Errorf("realtime is disabled for this user")
	}
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*") // For CORS support
	sessionId := r.URL.Query().Get("sessionId")
	username := d.user.Username
	// Check if the writer supports flushing
	f, ok := w.(http.Flusher)
	if !ok {
		// Log the issue for debugging purposes
		logger.Debug(fmt.Sprintf("error: ResponseWriter does not support Flusher. User: %s, SessionId: %s", username, sessionId))
		return http.StatusInternalServerError, fmt.Errorf("streaming not supported")
	}
	msgr := messenger{flusher: f, writer: w}

	// Listen for messages and client disconnection
	clientGone := r.Context().Done()
	err := msgr.sendEvent("acknowledge", "connection established")
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error sending message: %v, user: %s, SessionId: %s", err, username, sessionId)
	}
	for {
		select {
		case <-d.ctx.Done():
			err := msgr.sendEvent("notification", "server is shutting down, terminating connection.")
			if err != nil {
				return http.StatusInternalServerError, fmt.Errorf("error sending message: %v, user: %s, SessionId: %s", err, username, sessionId)
			}
			return http.StatusOK, nil
		case <-clientGone:
			logger.Debug(fmt.Sprintf("client disconnected. user: %s, SessionId: %s", username, sessionId))
			return http.StatusOK, nil
		case msg := <-events.BroadcastChan:
			err := msgr.sendEvent(msg.EventType, msg.Message)
			if err != nil {
				return http.StatusInternalServerError, fmt.Errorf("error sending message: %v, user: %s, SessionId: %s", err, username, sessionId)
			}
		}
	}
}
