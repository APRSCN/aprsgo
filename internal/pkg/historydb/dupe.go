package historydb

import (
	"hash/fnv"
	"strings"
	"sync"
	"time"
)

// DupeChecker suppresses duplicate packets seen within a sliding time window.
// It is safe for concurrent use.
//
// Duplicate detection is based on the packet's source callsign and information
// field only — the via path (and any q-construct appended to it) is ignored,
// so the same payload relayed through different paths is recognised as one
// packet. To also catch copies that a client has lightly corrupted in transit,
// each accepted packet is additionally stored under several normalised
// variants (trailing spaces removed, high bit handled, low/DEL bytes handled);
// a later packet matching any stored variant is treated as a duplicate.
type DupeChecker struct {
	mu        sync.Mutex
	seen      map[uint64]time.Time
	window    time.Duration
	lastSweep time.Time // last time expired entries were pruned
}

// NewDupeChecker creates a checker that treats identical packets as duplicates
// if seen again within window.
func NewDupeChecker(window time.Duration) *DupeChecker {
	return &DupeChecker{
		seen:   make(map[uint64]time.Time),
		window: window,
	}
}

// dupeKey reduces a raw packet line to the bytes used for duplicate detection:
// the source callsign (up to the first '>') joined with the information field
// (everything after the first ':'). The via path between them is dropped so a
// packet relayed via different routes hashes the same. If the line has no ':'
// the whole line is used as a fallback.
func dupeKey(packet string) string {
	src := packet
	if i := strings.IndexByte(packet, '>'); i >= 0 {
		src = packet[:i]
	}
	info := ""
	if i := strings.IndexByte(packet, ':'); i >= 0 {
		info = packet[i+1:]
	} else {
		// No information field separator: fall back to the full line so we
		// still suppress exact repeats.
		return packet
	}
	return src + ":" + info
}

// hashKey returns the 64-bit FNV-1a hash of s.
func hashKey(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

// mangleVariants returns the set of normalised variants of key (excluding key
// itself) that lightly-corrupted copies might arrive as. Only variants that
// actually differ from key are returned.
func mangleVariants(key string) []string {
	var out []string
	seen := map[string]struct{}{key: {}}
	add := func(s string) {
		if _, dup := seen[s]; dup {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	b := []byte(key)

	// Trailing spaces removed.
	add(strings.TrimRight(key, " "))

	// High-bit handling: strip 8-bit bytes / clear high bit / replace with space.
	var strip8, clear8, space8 []byte
	hasHigh := false
	for _, c := range b {
		if c&0x80 != 0 {
			hasHigh = true
			clear8 = append(clear8, c&0x7f)
			space8 = append(space8, ' ')
			// strip8: drop the byte entirely
		} else {
			strip8 = append(strip8, c)
			clear8 = append(clear8, c)
			space8 = append(space8, c)
		}
	}
	if hasHigh {
		add(string(strip8))
		add(string(clear8))
		add(string(space8))
	}

	// Low control bytes (0x01..0x1f): strip / replace with space.
	var lowStrip, lowSpace []byte
	hasLow := false
	for _, c := range b {
		if c < 0x20 && c > 0 {
			hasLow = true
			lowSpace = append(lowSpace, ' ')
			// lowStrip: drop
		} else {
			lowStrip = append(lowStrip, c)
			lowSpace = append(lowSpace, c)
		}
	}
	if hasLow {
		add(string(lowStrip))
		add(string(lowSpace))
	}

	// DEL bytes (0x7f): strip / replace with space.
	var delStrip, delSpace []byte
	hasDel := false
	for _, c := range b {
		if c == 0x7f {
			hasDel = true
			delSpace = append(delSpace, ' ')
			// delStrip: drop
		} else {
			delStrip = append(delStrip, c)
			delSpace = append(delSpace, c)
		}
	}
	if hasDel {
		add(string(delStrip))
		add(string(delSpace))
	}

	return out
}

// Seen reports whether packet was seen within the window; otherwise it records
// it (and its normalised variants) and returns false. Expired entries are
// pruned at most once per window (time-sampled) so the hot path stays O(1)
// amortised instead of scanning the whole map on every packet.
func (d *DupeChecker) Seen(packet string) bool {
	key := dupeKey(packet)
	keyHash := hashKey(key)

	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()

	// Exact (canonical) match within the window?
	if t, ok := d.seen[keyHash]; ok && now.Sub(t) < d.window {
		return true
	}

	// Prune expired entries at most once per window.
	if now.Sub(d.lastSweep) >= d.window {
		d.pruneLocked(now)
	}

	// Compute the normalised variants once and reuse for both the match check
	// and the store below.
	variants := mangleVariants(key)

	// Variant match within the window? (A previously stored packet whose
	// normalised form equals this one's canonical key.)
	for _, v := range variants {
		vh := hashKey(v)
		if t, ok := d.seen[vh]; ok && now.Sub(t) < d.window {
			return true
		}
	}

	// Not a duplicate: record the canonical key and all its variants so a
	// later corrupted copy is recognised.
	d.seen[keyHash] = now
	for _, v := range variants {
		d.seen[hashKey(v)] = now
	}
	return false
}

// Cleanup removes all entries older than the window. It is safe for concurrent
// use and intended to be called periodically (e.g. from cron) for long-lived
// checkers so the map does not retain expired keys indefinitely.
func (d *DupeChecker) Cleanup() {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pruneLocked(now)
}

// pruneLocked deletes expired entries and records the sweep time. The caller
// must hold d.mu.
func (d *DupeChecker) pruneLocked(now time.Time) {
	for k, t := range d.seen {
		if now.Sub(t) >= d.window {
			delete(d.seen, k)
		}
	}
	d.lastSweep = now
}
