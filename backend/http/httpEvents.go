package http

import (
	"fmt"
	"io"
	"net/http"

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
	if !d.user.Permissions.Realtime {
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
			logger.Debugf("client disconnected. user: %s, SessionId: %s", username, sessionId)
			return http.StatusOK, nil

		case msg := <-events.BroadcastChan:
			if err := msgr.sendEvent(msg.EventType, msg.Message); err != nil {
				return http.StatusInternalServerError, fmt.Errorf("error sending broadcast: %v, user: %s", err, username)
			}

		case msg, ok := <-sendChan:
			if !ok {
				logger.Debugf("SSE channel closed for user: %s, SessionId: %s", username, sessionId)
				return http.StatusOK, nil
			}
			if err := msgr.sendEvent(msg.EventType, msg.Message); err != nil {
				return http.StatusInternalServerError, fmt.Errorf("error sending targeted message: %v, user: %s", err, username)
			}
		}
	}
}
