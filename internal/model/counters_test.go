package model

import (
	"sync"
	"testing"
)

func TestCountersSnapshot(t *testing.T) {
	var c Counters
	c.AddSentPackets(3)
	c.AddReceivedPackets(5)
	c.AddReceivedDups(2)
	c.AddReceivedErrors(1)
	c.AddReceivedQDrop(4)
	c.AddSentBytes(100)
	c.AddReceivedBytes(200)

	s := c.Snapshot()
	if s.SentPackets != 3 || s.ReceivedPackets != 5 || s.ReceivedDups != 2 ||
		s.ReceivedErrors != 1 || s.ReceivedQDrop != 4 || s.SentBytes != 100 || s.ReceivedBytes != 200 {
		t.Errorf("snapshot mismatch: %+v", s)
	}
}

func TestCountersRates(t *testing.T) {
	var c Counters
	c.AddReceivedPackets(10)
	c.AddSentBytes(50)
	c.UpdateRates() // first tick: rate equals the delta from zero
	if s := c.Snapshot(); s.RecvPacketRate != 10 || s.SendByteRate != 50 {
		t.Errorf("first rates wrong: recv=%d sendBytes=%d", s.RecvPacketRate, s.SendByteRate)
	}
	c.AddReceivedPackets(3)
	c.UpdateRates() // second tick: only the new delta
	if s := c.Snapshot(); s.RecvPacketRate != 3 {
		t.Errorf("second rate wrong: recv=%d, want 3", s.RecvPacketRate)
	}
}

// TestCountersConcurrent exercises concurrent atomic increments; run with -race.
// (UpdateRates is documented as single-goroutine, so it is only called after the
// concurrent writers finish.)
func TestCountersConcurrent(t *testing.T) {
	var c Counters
	const goroutines = 50
	const each = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < each; j++ {
				c.AddReceivedPackets(1)
				c.AddReceivedBytes(2)
			}
		}()
	}
	wg.Wait()

	c.UpdateRates()
	s := c.Snapshot()
	if want := uint64(goroutines * each); s.ReceivedPackets != want {
		t.Errorf("ReceivedPackets = %d, want %d", s.ReceivedPackets, want)
	}
	if want := uint64(goroutines * each * 2); s.ReceivedBytes != want {
		t.Errorf("ReceivedBytes = %d, want %d", s.ReceivedBytes, want)
	}
}
