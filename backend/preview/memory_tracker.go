package preview

import (
	"context"
	"sync/atomic"
)

// MemoryTracker wraps the existing FFmpegService to add memory tracking
type MemoryTracker struct {
	maxMemoryBytes  int64
	usedMemoryBytes int64
	semaphore       chan struct{}
}

// NewMemoryTracker creates a memory-aware concurrency limiter
func NewMemoryTracker(maxConcurrent int, maxMemoryMB int) *MemoryTracker {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	if maxMemoryMB < 50 {
		maxMemoryMB = 500 // Default 500MB
	}
	return &MemoryTracker{
		maxMemoryBytes: int64(maxMemoryMB) * 1024 * 1024,
		semaphore:      make(chan struct{}, maxConcurrent),
	}
}

// TryAcquire attempts to acquire resources for processing an image
// Returns false if memory limit would be exceeded
func (mt *MemoryTracker) TryAcquire(ctx context.Context, estimatedBytes int64) bool {
	// Check if we have memory headroom
	currentUsed := atomic.LoadInt64(&mt.usedMemoryBytes)
	if currentUsed+estimatedBytes > mt.maxMemoryBytes {
		return false
	}

	// Try to acquire semaphore slot (non-blocking check first)
	select {
	case mt.semaphore <- struct{}{}:
		// Successfully acquired slot, now add memory
		atomic.AddInt64(&mt.usedMemoryBytes, estimatedBytes)
		return true
	case <-ctx.Done():
		return false
	default:
		return false
	}
}

// Acquire waits to acquire resources
func (mt *MemoryTracker) Acquire(ctx context.Context, estimatedBytes int64) error {
	// Wait for semaphore slot
	select {
	case mt.semaphore <- struct{}{}:
		atomic.AddInt64(&mt.usedMemoryBytes, estimatedBytes)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release frees up resources
func (mt *MemoryTracker) Release(estimatedBytes int64) {
	<-mt.semaphore
	atomic.AddInt64(&mt.usedMemoryBytes, -estimatedBytes)
}

// GetMemoryUsageMB returns current memory usage in MB
func (mt *MemoryTracker) GetMemoryUsageMB() int64 {
	return atomic.LoadInt64(&mt.usedMemoryBytes) / (1024 * 1024)
}

// estimateImageMemoryBytes estimates memory needed for image processing
// Returns bytes needed for decoded RGBA image + processing overhead
func estimateImageMemoryBytes(width, height int) int64 {
	// Decoded RGBA: width * height * 4 bytes
	// Processing overhead (temporary buffers during resize): 2x
	// Total = pixels * 4 * 2
	pixels := int64(width) * int64(height)
	bytes := pixels * 4 * 2
	
	// Minimum 1MB for small images
	if bytes < 1024*1024 {
		bytes = 1024 * 1024
	}
	
	return bytes
}
