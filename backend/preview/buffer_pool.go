package preview

import (
	"bytes"
	"sync"
)

// bufferPool manages reusable buffers to reduce allocations
var bufferPool = sync.Pool{
	New: func() interface{} {
		// Create 128KB buffers by default
		return bytes.NewBuffer(make([]byte, 0, 128*1024))
	},
}

// getBuffer gets a buffer from the pool
func getBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// putBuffer returns a buffer to the pool
func putBuffer(buf *bytes.Buffer) {
	// Don't pool buffers that grew too large (>1MB)
	if buf.Cap() > 1024*1024 {
		return
	}
	buf.Reset()
	bufferPool.Put(buf)
}
