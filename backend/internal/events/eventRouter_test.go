package events

import (
	"sync"
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

func TestConcurrentSSEClientLifecycleRace(t *testing.T) {
	stop := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			user := "race-user-" + string(rune('a'+id))
			source := "race-source-" + string(rune('a'+id%2))
			for {
				select {
				case <-stop:
					return
				default:
					ch := Register(user, []string{source, "shared-source"})
					SendSourceUpdate(source, "tick")
					SendSourceUpdate("shared-source", "tick")
					SendToUsers("ping", "msg", []string{user})
					Unregister(user, ch)
				}
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				SendSourceUpdate("shared-source", "broadcast")
				SendToUsers("broadcast", "all", []string{"race-user-a", "race-user-b"})
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)
	Shutdown()
	close(stop)
	wg.Wait()
	time.Sleep(50 * time.Millisecond)
}
