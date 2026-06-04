package uplink

import (
	"sync"
	"testing"
	"time"

	config2 "github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"go.uber.org/zap"
)

// emptyUplinkConfig returns a config with no uplinks, so the managers run idle
// and Init/Reload/Stop exercise the lifecycle without any network I/O.
func emptyUplinkConfig() config2.StaticConfig {
	var c config2.StaticConfig
	c.Server.ID = "TESTING"
	c.Server.BuffSize = 128
	return c
}

// TestUplinkConcurrentReload drives Init, repeated Reload and Stop concurrently
// with status readers that touch the stop channel and the active-client map.
// Its purpose is to catch data races on the package-level stop channel (the
// re-arm in Reload vs. the reads in the manager/stats goroutines) under the
// race detector; it asserts only that the lifecycle completes without hanging.
func TestUplinkConcurrentReload(t *testing.T) {
	logger.L = zap.NewNop()
	config2.Set(emptyUplinkConfig())

	Init()
	t.Cleanup(Stop)

	const readers = 4
	stopReaders := make(chan struct{})
	var wg sync.WaitGroup

	// Concurrent readers that race the Reload re-arm of stopCh and the active
	// map: GetClient/Clients take clientMu; currentStop takes stopMu.
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopReaders:
					return
				default:
					_ = GetClient()
					_ = Clients()
					_ = currentStop()
				}
			}
		}()
	}

	// Repeatedly reload; each Reload calls Stop (closeStop + mgrWG.Wait) then
	// armStop + relaunch, which is exactly the path that previously raced the
	// goroutines' read of the package stopCh variable.
	for i := 0; i < 20; i++ {
		Reload()
	}

	close(stopReaders)
	wg.Wait()

	// A final Stop (also via t.Cleanup) must not block; verify it returns
	// promptly so a lifecycle regression surfaces as a test timeout here.
	done := make(chan struct{})
	go func() {
		Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return within 5s after concurrent reloads")
	}
}
