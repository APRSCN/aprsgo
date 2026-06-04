package listener

import (
	"sync"
	"time"

	"github.com/APRSCN/aprsgo/internal/model"
)

// Client provides a struct to record client
type Client struct {
	At       string
	Port     int
	ID       string
	Verified bool
	Addr     string
	Uptime   time.Time
	Last     time.Time
	Software string
	Version  string
	Filter   string
	OutQ     int
	MsgRcpts int

	Stats model.Statistics
}

// Clients records all clients. It is replaced wholesale every second by the
// update goroutine; readers must go through ClientsSnapshot (or hold
// ClientsMutex) to avoid racing that replacement.
var Clients = make(map[any]*Client)

// ClientsMutex is the operation lock of clients
var ClientsMutex sync.RWMutex

// ClientsSnapshot returns the current client list as a slice, read under the
// lock so callers (e.g. the status handler) never race the update goroutine
// that replaces the Clients map.
func ClientsSnapshot() []*Client {
	ClientsMutex.RLock()
	defer ClientsMutex.RUnlock()
	out := make([]*Client, 0, len(Clients))
	for _, c := range Clients {
		out = append(out, c)
	}
	return out
}

// heardLen returns the number of stations a client has heard, tolerating a nil
// heard list.
func heardLen(c *TCPAPRSClient) int {
	if c.heard == nil {
		return 0
	}
	return c.heard.Len()
}

// update client list
func update() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		newClients := make(map[any]*Client)
		for _, l := range snapshotListeners() {
			// Only TCP listeners maintain a client map.
			if l.s == nil {
				continue
			}
			l.s.mu.RLock()
			addr := ""
			if l.s.listener != nil {
				addr = l.s.listener.Addr().String()
			}
			for c := range l.s.clients {
				// Read the client's mutable fields under its own mutex to
				// get a consistent snapshot (callsign/heard/conn are all
				// written while holding c.mu).
				c.mu.Lock()
				if c.conn == nil {
					c.mu.Unlock()
					continue
				}
				newClients[c] = &Client{
					At:       addr,
					Port:     l.Port,
					ID:       c.callSign,
					Verified: c.verified,
					Addr:     c.conn.RemoteAddr().String(),
					Uptime:   c.uptime,
					Last:     c.lastActive,
					Software: c.software,
					Version:  c.version,
					Filter:   c.filter,
					// OutQ: bytes currently queued for delivery to the
					// client (real async output-queue backlog).
					OutQ:     int(c.outQBytes.Load()),
					MsgRcpts: heardLen(c),
					Stats:    c.stats.Snapshot(),
				}
				c.mu.Unlock()
			}
			l.s.mu.RUnlock()
		}
		ClientsMutex.Lock()
		Clients = newClients
		ClientsMutex.Unlock()
	}
}
