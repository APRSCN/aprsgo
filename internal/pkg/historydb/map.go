package historydb

import (
	"sync"
	"time"
)

// MapFloat64History is the basic struct to record history with float64 map
type MapFloat64History struct {
	l *sync.RWMutex
	D map[any]float64
}

// NewMapFloat64History creates a new history map with float64
func NewMapFloat64History() *MapFloat64History {
	return &MapFloat64History{
		l: new(sync.RWMutex),
		D: make(map[any]float64),
	}
}

// Record records data
func (d *MapFloat64History) Record(key any, value float64) {
	d.l.Lock()
	defer d.l.Unlock()
	d.D[key] = value
}

// ClearByKey clears expired data by key as time
func (d *MapFloat64History) ClearByKey(TTL float64) {
	d.l.Lock()
	defer d.l.Unlock()
	// Get time now
	now := time.Now()

	for k := range d.D {
		if k.(float64)+TTL <= float64(now.UnixNano())/1e9 {
			delete(d.D, k)
		}
	}
}

// ToSlice transfers map to slice
func (d *MapFloat64History) ToSlice() [][2]any {
	d.l.RLock()
	defer d.l.RUnlock()
	ret := make([][2]any, len(d.D))
	count := 0
	for k, v := range d.D {
		ret[count] = [2]any{k, v}
		count++
	}
	return ret
}
