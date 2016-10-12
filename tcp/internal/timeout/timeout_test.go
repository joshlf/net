package timeout

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"
)

// NOTE(joshlf): Make sure to run this test with the race detector on

func TestTimeout(t *testing.T) {
	// The point of this test is to make sure that every timeout callback
	// is called at most once, and those that are cancelled are never
	// called.

	var mu sync.Mutex
	type timeoutTest struct {
		calls     int
		cancelled bool
		f         func()
	}
	makeTimeoutTest := func() *timeoutTest {
		var tt timeoutTest
		tt.f = func() {
			if tt.cancelled {
				t.Fatalf("cancelled timeout callback called")
			}
			tt.calls++
			if tt.calls > 1 {
				t.Fatalf("timeout callback called more than once")
			}
		}
		return &tt
	}
	timeoutTests := make(map[*timeoutTest]*Timeout)

	// run the test for one second
	end := NowMonotonic().Add(time.Second)
	numgoroutines := 4 // minimum
	if runtime.NumCPU() > numgoroutines {
		numgoroutines = runtime.NumCPU()
	}

	daemon := NewDaemon(&mu)
	var wg sync.WaitGroup
	wg.Add(numgoroutines)
	for i := 0; i < numgoroutines; i++ {
		go func() {
			defer wg.Done()
			for {
				if NowMonotonic().After(end) {
					return
				}

				if rand.Int()%100 == 0 {
					// 1 in 100 chance of cancelling existing timeout;
					// randomly pick one timeout and cancel it
					mu.Lock()
					for _, to := range timeoutTests {
						to.Cancel()
						break
					}
					mu.Unlock()
				} else {
					// 99 in 100 chance of spawning a new timeout
					tt := makeTimeoutTest()
					to := daemon.AddTimeout(tt.f, NowMonotonic().Add(time.Millisecond*10))
					mu.Lock()
					timeoutTests[tt] = to
					mu.Unlock()
				}

			}
		}()
	}

	wg.Wait()
}

func TestTimeoutLiveness(t *testing.T) {
	// The point of this test is to make sure that every non-cancelled
	// timeout is eventually called

	var mu sync.Mutex
	daemon := NewDaemon(&mu)

	// access the counter without synchronization
	// from the timeout callbacks; this will give
	// the race detector a chance to detect a race
	// if the normal synchronization isn't functioning
	// properly
	var counter int
	messages := make([]string, 0, 3)
	var wg sync.WaitGroup
	wg.Add(3)
	for i := 0; i < 3; i++ {
		target := NowMonotonic().Add(time.Millisecond * 10 * time.Duration(i))
		f := func() {
			counter++
			diff := NowMonotonic().Sub(target) // how late we were
			if diff < 0 {
				messages = append(messages, fmt.Sprintf("callback executed %v before target", diff))
			} else {
				messages = append(messages, fmt.Sprintf("callback executed %v after target", diff))
			}
			wg.Done()
		}
		daemon.AddTimeout(f, target)
	}

	wg.Wait()
	if counter != 3 {
		t.Errorf("unexpected counter: got %v; want %v", counter, 3)
	}
	for _, msg := range messages {
		fmt.Println(msg)
	}
}
