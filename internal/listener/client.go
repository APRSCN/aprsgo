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
var ClientsMutex sync.RWMutex

func Get(key any) (*Client, bool) {
	ClientsMutex.RLock()
	defer ClientsMutex.RUnlock()
	v, ok := Clients[key]
	return v, ok
}

func GetWithoutOK(key any) *Client {
	ClientsMutex.RLock()
	defer ClientsMutex.RUnlock()
	return Clients[key]
}

func GetAll() map[any]*Client {
	ClientsMutex.RLock()
	defer ClientsMutex.RUnlock()
	return Clients
}

func Set(key any, v *Client) {
	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()
	Clients[key] = v
}

func Del(key any) {
	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()
	delete(Clients, key)
}
