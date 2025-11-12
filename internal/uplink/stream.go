package uplink

import (
	"sync"
)

// DataStream provides a basic struct to build data Stream
type DataStream struct {
	subscribers []chan string
	mu          sync.RWMutex
	bufferSize  int
}

// NewDataStream creates a new data Stream
func NewDataStream(bufferSize int) *DataStream {
	return &DataStream{
		subscribers: make([]chan string, 0),
		bufferSize:  bufferSize,
	}
}

// Write data to Stream
func (ds *DataStream) Write(data string) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	for _, ch := range ds.subscribers {
		select {
		case ch <- data:
		default:
			// Skip full chan
		}
	}
}

// Subscribe a Stream
func (ds *DataStream) Subscribe() (<-chan string, func()) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ch := make(chan string, ds.bufferSize)
	ds.subscribers = append(ds.subscribers, ch)

	unsubscribe := func() {
		ds.mu.Lock()
		defer ds.mu.Unlock()
		for i, subscriber := range ds.subscribers {
			if subscriber == ch {
				// Remove from slice
				ds.subscribers = append(ds.subscribers[:i], ds.subscribers[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, unsubscribe
}
