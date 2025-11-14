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

	c *TCPAPRSClient // For closing

	Stats model.Statistics
}

// Clients records all clients
var Clients = make(map[any]*Client)

// ClientsMutex is the operation lock of clients
var ClientsMutex sync.RWMutex

// Get a client with OK
func Get(key any) (*Client, bool) {
	ClientsMutex.RLock()
	defer ClientsMutex.RUnlock()
	v, ok := Clients[key]
	return v, ok
}

// GetWithoutOK gets a client without OK
func GetWithoutOK(key any) *Client {
	ClientsMutex.RLock()
	defer ClientsMutex.RUnlock()
	return Clients[key]
}

// GetAll gets all clients
func GetAll() map[any]*Client {
	ClientsMutex.RLock()
	defer ClientsMutex.RUnlock()
	return Clients
}

// Set a client
func Set(key any, v *Client) {
	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()
	Clients[key] = v
}

// Del a client
func Del(key any) {
	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()
	delete(Clients, key)
}
