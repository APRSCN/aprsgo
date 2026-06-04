package historydb

import (
	"strings"
	"sync"
	"time"
)

// posTTL is how long a station's last-known position is retained, used by the
// range filters (m/, f/, t/.../call/km).
const posTTL = 48 * time.Hour

// posEntry is a single station's last-known position.
type posEntry struct {
	lat, lon float64
	at       time.Time
}

// PositionHistory records the last-known position of stations by callsign so
// position-aware filters can resolve friend/own positions. It is safe for
// concurrent use.
type PositionHistory struct {
	mu sync.RWMutex
	d  map[string]posEntry
}

// NewPositionHistory creates an empty position history store.
func NewPositionHistory() *PositionHistory {
	return &PositionHistory{d: make(map[string]posEntry)}
}

// Positions is the process-wide station position store, shared by the packet
// ingestion path (which records positions) and the filter layer (which reads
// them for m/, f/ and ranged t/ filters).
var Positions = NewPositionHistory()

// normalise folds a callsign to a canonical case-insensitive key.
func normalise(call string) string {
	return strings.ToUpper(strings.TrimSpace(call))
}

// Update records (or refreshes) the position of a station. Calls with an empty
// callsign are ignored.
func (h *PositionHistory) Update(call string, lat, lon float64) {
	call = normalise(call)
	if call == "" {
		return
	}
	h.mu.Lock()
	h.d[call] = posEntry{lat: lat, lon: lon, at: time.Now()}
	h.mu.Unlock()
}

// Get returns the last-known position of a station. ok is false when the
// station is unknown or its record has expired.
func (h *PositionHistory) Get(call string) (lat, lon float64, ok bool) {
	call = normalise(call)
	h.mu.RLock()
	e, found := h.d[call]
	h.mu.RUnlock()
	if !found {
		return 0, 0, false
	}
	if time.Since(e.at) > posTTL {
		return 0, 0, false
	}
	return e.lat, e.lon, true
}

// Len returns the number of currently stored stations (including any not yet
// expired-out by Cleanup).
func (h *PositionHistory) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.d)
}

// Cleanup removes expired entries. It is intended to be called periodically.
func (h *PositionHistory) Cleanup() {
	cutoff := time.Now().Add(-posTTL)
	h.mu.Lock()
	defer h.mu.Unlock()
	for k, e := range h.d {
		if e.at.Before(cutoff) {
			delete(h.d, k)
		}
	}
}
