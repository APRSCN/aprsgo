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

// ClearByValue clears expired data by value as time
func (d *MapFloat64History) ClearByValue(TTL float64) {
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

// Contain returns whether the record contain the key
func (d *MapFloat64History) Contain(key any) bool {
	d.l.RLock()
	defer d.l.RUnlock()
	_, ok := d.D[key]
	return ok
}

// Get returns value from the record
func (d *MapFloat64History) Get(key any) (float64, bool) {
	d.l.RLock()
	defer d.l.RUnlock()
	v, ok := d.D[key]
	return v, ok
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
