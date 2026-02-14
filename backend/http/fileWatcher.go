package http

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/events"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

// fileWatchResponse represents the response from file watch
type fileWatchResponse struct {
	Contents string             `json:"contents,omitempty"` // Text content for text files
	IsText   bool               `json:"isText"`             // Whether the file is a text file
	Metadata *fileWatchMetadata `json:"metadata,omitempty"` // File metadata for non-text files
}

// fileWatchMetadata contains file information for non-text files
type fileWatchMetadata struct {
	Name     string    `json:"name"`     // File name
	Size     int64     `json:"size"`     // File size in bytes (for directories, total size of all files)
	Type     string    `json:"type"`     // MIME type
	Modified time.Time `json:"modified"` // Modification time
}

// readLastNLines reads the last N lines from a file efficiently
func readLastNLines(filePath string, n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("number of lines must be positive")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	fileSize := stat.Size()
	if fileSize == 0 {
		return "", nil
	}

	// For small files, just read everything
	// For larger files, read from the end
	const maxReadSize = 1024 * 1024 // 1MB
	var lines []string

	if fileSize <= maxReadSize {
		// Read entire file for small files
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return "", err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return "", err
		}
	} else {
		// For large files, read from the end
		// Read the last chunk (up to 1MB) and count newlines
		readSize := maxReadSize
		if fileSize < int64(readSize) {
			readSize = int(fileSize)
		}

		if _, err := file.Seek(-int64(readSize), io.SeekEnd); err != nil {
			return "", err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return "", err
		}
	}

	// Return only the last N lines
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	// Truncate lines that exceed 250 characters
	const maxLineLength = 250
	for i, line := range lines {
		if len(line) > maxLineLength {
			lines[i] = line[:maxLineLength] + "..."
		}
	}

	return strings.Join(lines, "\n"), nil
}

// fileWatchHandler handles file watching requests
// @Summary Watch a file
// @Description Returns the last N lines of a file
// @Tags Tools
// @Accept json
// @Produce json
// @Param path query string true "Path to the file"
// @Param source query string true "Source name"
// @Param lines query int false "Number of lines to read (default: 10, max: 50)"
// @Param latencyCheck query bool false "Return minimal response for latency checking"
// @Success 200 {object} fileWatchResponse
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 403 {object} map[string]string "Permission denied"
// @Failure 404 {object} map[string]string "File not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/tools/watch [get]
func fileWatchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// Check for latency check request - return immediately with minimal response
	if r.URL.Query().Get("latencyCheck") != "" {
		return http.StatusOK, nil
	}

	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	linesStr := r.URL.Query().Get("lines")

	if path == "" || source == "" {
		return http.StatusBadRequest, fmt.Errorf("path and source are required")
	}
	var err error
	path, err = utils.SanitizeUserPath(path)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path: %v", err)
	}

	// Parse lines parameter
	lines := 10 // default
	if linesStr != "" {
		var parsedLines int
		parsedLines, err = strconv.Atoi(linesStr)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid lines parameter: %v", err)
		}
		if parsedLines < 1 || parsedLines > 50 {
			return http.StatusBadRequest, fmt.Errorf("lines must be between 1 and 50")
		}
		lines = parsedLines
	}

	// Validate user has access to the source
	userScope, err := d.user.GetScopeForSourceName(source)
	if err != nil {
		return http.StatusForbidden, err
	}

	// Check download permission (required to read file content)
	if !d.user.Permissions.Download {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to read file content")
	}

	// Resolve the full path
	scopePath := utils.JoinPathAsUnix(userScope, path)

	// Get the index for the source
	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source %s is not available", source)
	}

	// Check access control
	if store.Access != nil {
		if !store.Access.Permitted(idx.Path, scopePath, d.user.Username) {
			return http.StatusForbidden, fmt.Errorf("access denied to file")
		}
	}

	// Get real file path
	realPath, _, err := idx.GetRealPath(scopePath)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("file not found: %v", err)
	}

	// Get file/directory info
	info, err := os.Stat(realPath)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("path not found: %v", err)
	}

	// Get MIME type from the index if available
	mimeType := "application/octet-stream"
	reducedInfo, exists := idx.GetReducedMetadata(scopePath, false)
	if exists && reducedInfo.Type != "" {
		mimeType = reducedInfo.Type
	}

	response := fileWatchResponse{}
	// Handle directory - just return metadata, no content
	response.IsText = false
	response.Metadata = &fileWatchMetadata{
		Name:     info.Name(),
		Size:     0, // Directories don't have a meaningful size
		Type:     mimeType,
		Modified: info.ModTime(),
	}
	if !info.IsDir() {
		// Handle regular file
		// Check if file is a text file
		isText, err := utils.IsTextFile(realPath)
		if err != nil {
			return http.StatusInternalServerError, fmt.Errorf("error checking file type: %v", err)
		}
		if isText {
			// Read the last N lines for text files only
			content, err := readLastNLines(realPath, lines)
			if err != nil {
				return http.StatusInternalServerError, fmt.Errorf("error reading file: %v", err)
			}
			response.Contents = content
		}
	}

	w.Header().Set("Content-Type", "application/json")
	return http.StatusOK, json.NewEncoder(w).Encode(response)
}

// fileWatchSSEEvent represents the SSE event payload for file watch updates
type fileWatchSSEEvent struct {
	Contents string             `json:"contents,omitempty"` // Text content for text files
	IsText   bool               `json:"isText"`             // Whether the file is a text file
	Metadata *fileWatchMetadata `json:"metadata,omitempty"` // File metadata for non-text files
}

// fileWatchSSEHandler handles Server-Sent Events for file watching
// @Summary Watch a file via SSE
// @Description Establishes an SSE connection to receive periodic file updates
// @Tags Tools
// @Param path query string true "Path to the file"
// @Param source query string true "Source name"
// @Param lines query int false "Number of lines to read (default: 10, max: 50)"
// @Param interval query int false "Update interval in seconds (1, 2, 5, 10, 15, or 30, requires realtime permission for SSE)"
// @Success 200 "SSE stream"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 403 {object} map[string]string "Permission denied"
// @Failure 404 {object} map[string]string "File not found"
// @Router /api/tools/watch/sse [get]
func fileWatchSSEHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// Check realtime permissions
	if !(d.user.Permissions.Realtime) {
		return http.StatusForbidden, fmt.Errorf("realtime permission required for SSE file watching")
	}

	if !d.user.Permissions.Download {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to read file content")
	}

	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	linesStr := r.URL.Query().Get("lines")
	intervalStr := r.URL.Query().Get("interval")

	var err error
	if path == "" || source == "" {
		return http.StatusBadRequest, fmt.Errorf("path and source are required")
	}

	// Rule 1: Validate user-provided path to prevent path traversal
	cleanPath, err := utils.SanitizeUserPath(path)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path: %v", err)
	}
	path = cleanPath

	// Parse lines parameter
	lines := 10 // default
	if linesStr != "" {
		var parsedLines int
		parsedLines, err = strconv.Atoi(linesStr)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid lines parameter: %v", err)
		}
		if parsedLines < 1 || parsedLines > 50 {
			return http.StatusBadRequest, fmt.Errorf("lines must be between 1 and 50")
		}
		lines = parsedLines
	}

	// Parse interval parameter (valid intervals: 1, 2, 5, 10, 15, 30 seconds)
	interval := time.Second * 1 // default
	if intervalStr != "" {
		var parsedInterval int
		parsedInterval, err = strconv.Atoi(intervalStr)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid interval parameter: %v", err)
		}
		validIntervals := map[int]bool{1: true, 2: true, 5: true, 10: true, 15: true, 30: true}
		if !validIntervals[parsedInterval] {
			return http.StatusBadRequest, fmt.Errorf("interval must be one of: 1, 2, 5, 10, 15, 30 seconds")
		}
		interval = time.Duration(parsedInterval) * time.Second
	}

	// Validate user has access to the source
	userScope, err := d.user.GetScopeForSourceName(source)
	if err != nil {
		return http.StatusForbidden, err
	}
	// Resolve the full path
	scopePath := utils.JoinPathAsUnix(userScope, path)

	// Get the index for the source
	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source %s is not available", source)
	}

	// Check access control
	if store.Access != nil {
		if !store.Access.Permitted(idx.Path, scopePath, d.user.Username) {
			return http.StatusForbidden, fmt.Errorf("access denied to file")
		}
	}

	// Get real file path
	realPath, _, err := idx.GetRealPath(scopePath)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("path not found: %v", err)
	}

	// Get file/directory info
	info, err := os.Stat(realPath)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("path not found: %v", err)
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("streaming not supported")
	}

	msgr := messenger{flusher: flusher, writer: w}
	clientGone := r.Context().Done()
	username := d.user.Username

	// Initial ack
	statusMsg, _ := json.Marshal(map[string]interface{}{"status": "connected"})
	if err := msgr.sendEvent("fileWatch", string(statusMsg)); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error sending initial message: %v", err)
	}

	// Register this client with the events system (like general SSE handler)
	sendChan := events.Register(username, []string{source})
	defer events.Unregister(username, sendChan)

	// Get MIME type (we'll reuse this)
	mimeType := "application/octet-stream"
	reducedInfo, exists := idx.GetReducedMetadata(scopePath, false)
	if exists && reducedInfo.Type != "" {
		mimeType = reducedInfo.Type
	}

	// Determine if this is a directory
	isDir := info.IsDir()

	// Start background goroutine to periodically send file updates via events system
	stopTicker := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Send initial update
		sendFileWatchUpdate(username, realPath, path, source, lines, mimeType, isDir)

		for {
			select {
			case <-stopTicker:
				return
			case <-ticker.C:
				sendFileWatchUpdate(username, realPath, path, source, lines, mimeType, isDir)
			}
		}
	}()

	// Main loop: listen for events from events system (like general SSE handler)
	// Use server context if available, otherwise use request context
	serverCtx := r.Context()
	if d.ctx != nil {
		serverCtx = d.ctx
	}

	for {
		select {
		case <-serverCtx.Done():
			close(stopTicker)
			_ = msgr.sendEvent("fileWatch", "\"server shutting down\"")
			return http.StatusOK, nil

		case <-clientGone:
			close(stopTicker)
			return http.StatusOK, nil

		case msg, ok := <-sendChan:
			if !ok {
				close(stopTicker)
				return http.StatusOK, nil
			}
			// Only process fileWatch events for this connection
			if msg.EventType == "fileWatch" {
				if err := msgr.sendEvent(msg.EventType, msg.Message); err != nil {
					close(stopTicker)
					return http.StatusInternalServerError, fmt.Errorf("error sending event: %v", err)
				}
			}
		}
	}
}

// sendFileWatchUpdate reads the file/directory and sends an update via the events system
func sendFileWatchUpdate(username, realPath, path, source string, lines int, mimeType string, isDir bool) {
	// Re-check path (in case it was deleted or changed)
	info, err := os.Stat(realPath)
	if err != nil {
		// Path no longer exists
		errorMsg, _ := json.Marshal(map[string]interface{}{"status": "error", "error": "path not found"})
		events.SendToUsers("fileWatch", string(errorMsg), []string{username})
		return
	}

	// Build the SSE event
	sseEvent := fileWatchSSEEvent{}

	if isDir {
		// Handle directory - just return metadata, no content
		sseEvent.IsText = false
		sseEvent.Metadata = &fileWatchMetadata{
			Name:     info.Name(),
			Size:     0, // Directories don't have a meaningful size
			Type:     "directory",
			Modified: info.ModTime(),
		}
	} else {
		// Handle regular file
		// Check if file is a text file
		var isText bool
		isText, err = utils.IsTextFile(realPath)
		if err != nil {
			// Error checking file, skip this update
			return
		}

		sseEvent.IsText = isText
		sseEvent.Metadata = &fileWatchMetadata{
			Name:     info.Name(),
			Size:     info.Size(),
			Type:     mimeType,
			Modified: info.ModTime(),
		}

		if isText {
			// Read the last N lines for text files only
			var content string
			content, err = readLastNLines(realPath, lines)
			if err != nil {
				// Error reading, skip this update
				return
			}
			sseEvent.Contents = content
		}
		// For non-text files, Contents stays empty - frontend displays metadata
	}

	// Serialize and send via events system
	eventJSON, err := json.Marshal(sseEvent)
	if err != nil {
		return
	}

	// Send via events system to this user (like OnlyOffice does)
	events.SendToUsers("fileWatch", string(eventJSON), []string{username})
}
