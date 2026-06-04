package historydb

import "testing"

func TestPositionHistoryBasic(t *testing.T) {
	h := NewPositionHistory()

	if _, _, ok := h.Get("N0CALL"); ok {
		t.Error("unknown station should not be found")
	}

	h.Update("N0CALL", 60.5, 25.1)
	lat, lon, ok := h.Get("N0CALL")
	if !ok {
		t.Fatal("station should be found after update")
	}
	if lat != 60.5 || lon != 25.1 {
		t.Errorf("got %f,%f want 60.5,25.1", lat, lon)
	}

	// Case-insensitive lookup.
	if _, _, ok := h.Get("n0call"); !ok {
		t.Error("lookup should be case-insensitive")
	}

	// Empty callsign ignored.
	h.Update("", 1, 2)
	if h.Len() != 1 {
		t.Errorf("Len = %d, want 1 (empty callsign must be ignored)", h.Len())
	}
}

func TestPositionHistoryCleanup(t *testing.T) {
	h := NewPositionHistory()
	h.Update("AA1AA", 1, 2)
	// Force expiry by back-dating the entry beyond the TTL.
	h.mu.Lock()
	e := h.d["AA1AA"]
	e.at = e.at.Add(-posTTL - 1)
	h.d["AA1AA"] = e
	h.mu.Unlock()

	if _, _, ok := h.Get("AA1AA"); ok {
		t.Error("expired entry should not be returned by Get")
	}

	h.Cleanup()
	if h.Len() != 0 {
		t.Errorf("Len = %d after cleanup, want 0", h.Len())
	}
}
