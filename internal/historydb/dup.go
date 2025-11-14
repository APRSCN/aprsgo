package historydb

import (
	"sync"
	"time"
)

type DupRecord struct {
	l *sync.RWMutex
	D map[uint64]float64
}

// NewDup creates a new dup record
func NewDup() *DupRecord {
	return &DupRecord{
		l: new(sync.RWMutex),
		D: make(map[uint64]float64),
	}
}

// Record records data
func (d *DupRecord) Record(hash uint64, time float64) {
	d.l.Lock()
	defer d.l.Unlock()
	d.D[hash] = time
}

// Clear clears expired data
func (d *DupRecord) Clear(TTL float64) {
	d.l.Lock()
	defer d.l.Unlock()
	// Get time now
	now := time.Now()

	for k, v := range d.D {
		if v+TTL <= float64(now.UnixNano())/1e9 {
			delete(d.D, k)
		}
	}
}

// Contain returns whether the record contain the hash
func (d *DupRecord) Contain(hash uint64) bool {
	d.l.RLock()
	defer d.l.RUnlock()
	_, ok := d.D[hash]
	return ok
}
