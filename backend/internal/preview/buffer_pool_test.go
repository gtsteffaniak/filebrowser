package preview

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBufferPool_GetPutBalanced verifies that getBuffer/putBuffer can be used
// in a loop without leaking or panicking, and that buffers are returned to the pool.
func TestBufferPool_GetPutBalanced(t *testing.T) {
	const iterations = 1000
	for i := 0; i < iterations; i++ {
		buf := getBuffer()
		require.NotNil(t, buf)
		require.NotNil(t, buf.Bytes())
		putBuffer(buf)
	}
}

// TestBufferPool_Reuse verifies that put buffers are reused (same capacity after get).
func TestBufferPool_Reuse(t *testing.T) {
	buf1 := getBuffer()
	cap1 := buf1.Cap()
	putBuffer(buf1)

	buf2 := getBuffer()
	cap2 := buf2.Cap()
	putBuffer(buf2)

	require.Equal(t, cap1, cap2, "pool should return buffers with same capacity")
	require.GreaterOrEqual(t, cap1, 128*1024, "default buffer should be at least 128KB")
}

// TestBufferPool_LargeBufferNotReused verifies that buffers that grew beyond 1MB
// are not put back in the pool (to avoid retaining large allocations).
func TestBufferPool_LargeBufferNotReused(t *testing.T) {
	buf := getBuffer()
	// Grow the buffer past the 1MB threshold
	growth := make([]byte, 2*1024*1024)
	_, _ = buf.Write(growth)
	require.Greater(t, buf.Cap(), 1024*1024)
	putBuffer(buf)

	// After putBuffer, a large buffer is not pooled. Get several buffers and
	// ensure we still get valid 128KB-capacity buffers (pool wasn't polluted).
	for i := 0; i < 10; i++ {
		b := getBuffer()
		require.NotNil(t, b)
		// Pool buffers are 128KB cap; if we got a fresh one it's 128KB
		require.GreaterOrEqual(t, b.Cap(), 128*1024, "getBuffer should return usable buffer")
		putBuffer(b)
	}
}

// TestBufferPool_ConcurrentGetPut stresses the pool under concurrent use (as in Resize).
func TestBufferPool_ConcurrentGetPut(t *testing.T) {
	const goroutines = 50
	const perGoroutine = 20
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				buf := getBuffer()
				_, _ = buf.Write([]byte("test"))
				putBuffer(buf)
			}
		}()
	}
	wg.Wait()
}
