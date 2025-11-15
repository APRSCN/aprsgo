package listener

import (
	"sync"
	"time"

	"github.com/APRSCN/aprsgo/internal/model"
)

// Client provides a struct to record client
type Client struct {
	At       string
	ID       string
	Verified bool
	Addr     string
	Uptime   time.Time
	Last     time.Time
	Software string
	Version  string
	Filter   string

	Stats model.Statistics
}

// Clients records all clients
var Clients = make(map[any]*Client)

// ClientsMutex is the operation lock of clients
var ClientsMutex sync.RWMutex

// update client list
func update() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			newClients := make(map[any]*Client)
			for _, l := range Listeners {
				for c := range l.s.clients {
					newClients[c] = &Client{
						At:       l.s.listener.Addr().String(),
						ID:       c.callSign,
						Verified: c.verified,
						Addr:     c.conn.RemoteAddr().String(),
						Uptime:   c.uptime,
						Last:     c.lastActive,
						Software: c.software,
						Version:  c.version,
						Filter:   c.filter,
						Stats:    *c.stats,
					}
				}
			}
			Clients = newClients
		}
	}
}
