package events

import (
	"testing"
	"time"
)

func TestConcurrentRegisterUnregisterSourceUpdates(t *testing.T) {
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				ch := Register("stress-user", []string{"stress-source"})
				Unregister("stress-user", ch)
			}
		}
	}()

	for i := 0; i < 200; i++ {
		SendSourceUpdate("stress-source", "tick")
	}
	close(stop)
	time.Sleep(50 * time.Millisecond)
}
