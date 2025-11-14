package uplink

import (
	"sync"
)

var Stream *DataStream

// StreamData is the basic struct for stream write
type StreamData struct {
	Data   string
	Writer string
}

// DataStream provides a basic struct to build data Stream
type DataStream struct {
	subscribers []chan StreamData
	mu          sync.RWMutex
	bufferSize  int
}

// NewDataStream creates a new data Stream
func NewDataStream(bufferSize int) *DataStream {
	return &DataStream{
		subscribers: make([]chan StreamData, 0),
		bufferSize:  bufferSize,
	}
}

// Write data to Stream
func (ds *DataStream) Write(data string, writer string) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	for _, ch := range ds.subscribers {
		select {
		case ch <- StreamData{
			Data:   data,
			Writer: writer,
		}:
		default:
			// Skip full chan
		}
	}
}

// Subscribe a Stream
func (ds *DataStream) Subscribe() (<-chan StreamData, func()) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ch := make(chan StreamData, ds.bufferSize)
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
