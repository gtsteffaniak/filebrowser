package preview

import (
	"bytes"
	"sync"
)

// bufferPool manages reusable buffers to reduce allocations
// Buffers start at 128KB (first retry level) and can grow up to 512KB
var bufferPool = sync.Pool{
	New: func() interface{} {
		// Create 128KB buffers by default to match first retry level
		return bytes.NewBuffer(make([]byte, 0, 128*1024))
	},
}

// getBuffer gets a buffer from the pool
func getBuffer() *bytes.Buffer {
	buf, ok := bufferPool.Get().(*bytes.Buffer)
	if !ok {
		// This should never happen, but provide a fallback
		buf = bytes.NewBuffer(make([]byte, 0, 128*1024))
	}
	buf.Reset()
	return buf
}

// putBuffer returns a buffer to the pool
func putBuffer(buf *bytes.Buffer) {
	// Don't pool buffers that grew too large (>1MB)
	// This prevents memory bloat from images with large headers (256KB/512KB retries)
	// Typical case: 128KB buffers are reused
	// Edge case: 256KB/512KB buffers are discarded and GC'd
	if buf.Cap() > 1024*1024 {
		return
	}
	buf.Reset()
	bufferPool.Put(buf)
}
