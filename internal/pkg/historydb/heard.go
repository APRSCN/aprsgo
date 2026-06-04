package historydb

import (
	"strings"
	"sync"
	"time"
)

// defaultHeardTTL is the fallback retention for a heard entry when a list is
// created without an explicit TTL. Message routing uses the heard list to
// decide whether a message addressed to a station should reach a given client.
const defaultHeardTTL = 30 * time.Minute

// maxHeard bounds the number of distinct stations tracked per client to avoid
// unbounded growth on very busy feeds.
const maxHeard = 4096

// HeardList records the set of source stations a client has recently received,
// each with a last-heard timestamp, expiring entries after a configurable TTL.
// It is safe for concurrent use.
type HeardList struct {
	mu  sync.Mutex
	d   map[string]time.Time
	ttl time.Duration
}

// NewHeardList creates an empty heard list with the default TTL.
func NewHeardList() *HeardList {
	return NewHeardListTTL(defaultHeardTTL)
}

// NewHeardListTTL creates an empty heard list whose entries expire after ttl.
// A non-positive ttl falls back to the default.
func NewHeardListTTL(ttl time.Duration) *HeardList {
	if ttl <= 0 {
		ttl = defaultHeardTTL
	}
	return &HeardList{d: make(map[string]time.Time), ttl: ttl}
}

// Add records that the client has just heard the given station. Empty
// callsigns are ignored; growth is bounded by maxHeard.
func (h *HeardList) Add(call string) {
	call = strings.ToUpper(strings.TrimSpace(call))
	if call == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.d[call]; !ok && len(h.d) >= maxHeard {
		// At capacity and this is a new station: drop the oldest entry.
		var oldestKey string
		var oldest time.Time
		for k, t := range h.d {
			if oldestKey == "" || t.Before(oldest) {
				oldestKey, oldest = k, t
			}
		}
		if oldestKey != "" {
			delete(h.d, oldestKey)
		}
	}
	h.d[call] = time.Now()
}

// Heard reports whether the client has heard the given station within the TTL.
func (h *HeardList) Heard(call string) bool {
	call = strings.ToUpper(strings.TrimSpace(call))
	if call == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	t, ok := h.d[call]
	if !ok {
		return false
	}
	if time.Since(t) > h.ttl {
		delete(h.d, call)
		return false
	}
	return true
}

// Take reports whether the given station is present and unexpired, removing it
// in that case so it matches at most once (one-shot consume). It is used to
// gate a single follow-up packet per recorded station.
func (h *HeardList) Take(call string) bool {
	call = strings.ToUpper(strings.TrimSpace(call))
	if call == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	t, ok := h.d[call]
	if !ok {
		return false
	}
	delete(h.d, call)
	return time.Since(t) <= h.ttl
}

// Len returns the number of stations currently tracked (including any not yet
// expired-out).
func (h *HeardList) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.d)
}
