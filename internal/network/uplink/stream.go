package uplink

import (
	"sync"

	"github.com/APRSCN/aprsgo/internal/pkg/historydb"
	"github.com/APRSCN/aprsutils/parser"
)

var Stream *DataStream

// Writer-tag conventions for StreamData.Writer. The writer identifies the
// origin of a packet so consumers can apply loop-prevention rules (do not echo
// a packet back to its source, keep uplink and peer traffic separate).
//
//   - A TCP/UDP/HTTP client's login callsign is used verbatim.
//   - WriterUplink marks traffic received from an upstream uplink.
//   - WriterPeerPrefix + "<id>" marks traffic received from a core peer.
const (
	// WriterUplink tags packets received from an upstream uplink.
	WriterUplink = "uplink"
	// WriterPeerPrefix is prepended to a peer's id to tag packets received
	// from that core peer ("peer:<id>").
	WriterPeerPrefix = "peer:"
)

// StreamData is the basic struct for stream write
type StreamData struct {
	Data parser.Parsed
	// Writer identifies the packet's origin; see the Writer-tag constants.
	Writer string
	// Dupe marks a packet that was detected as a duplicate. Such packets are
	// only delivered to dupefeed ports; normal clients skip them.
	Dupe bool
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

// Write data to Stream.
//
// This is the single choke point through which every accepted packet flows, so
// it is also where we record station positions for position-aware filters
// (m/, f/, ranged t/).
func (ds *DataStream) Write(data parser.Parsed, writer string) {
	// Record last-known position for the source station (and the inner source
	// of third-party traffic) so range filters can resolve it.
	recordPosition(&data)

	ds.broadcast(StreamData{Data: data, Writer: writer})
}

// WriteDupe publishes a packet flagged as a duplicate. Only dupefeed ports
// consume it; it does not update position history or reach normal clients.
func (ds *DataStream) WriteDupe(data parser.Parsed, writer string) {
	ds.broadcast(StreamData{Data: data, Writer: writer, Dupe: true})
}

// broadcast delivers a stream item to all current subscribers (non-blocking).
func (ds *DataStream) broadcast(item StreamData) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	for _, ch := range ds.subscribers {
		select {
		case ch <- item:
		default:
			// Skip full chan
		}
	}
}

// recordPosition stores a station's position in the shared position history.
func recordPosition(p *parser.Parsed) {
	if p.HasPosition && p.From != "" {
		historydb.Positions.Update(p.From, p.Lat, p.Lon)
	}
	// Objects/items carry their own name; record their position under it too.
	if p.ObjectName != "" && p.HasPosition {
		historydb.Positions.Update(p.ObjectName, p.Lat, p.Lon)
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
