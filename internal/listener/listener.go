package listener

import (
	"fmt"
	"time"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsutils/client"
	"github.com/ghinknet/json"
	"go.uber.org/zap"
)

// Listener provides a struct to record listener
type Listener struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Visible  string `json:"visible"`
	Filter   string `json:"filter"`

	s *TCPAPRSServer // For closing

	Stats model.Statistics
}

// Listeners records all listeners
var Listeners = make(map[any]Listener)

// Client provides a struct to record client
type Client struct {
	At       string    `json:"at"`
	ID       string    `json:"id"`
	Addr     string    `json:"addr"`
	Uptime   time.Time `json:"uptime"`
	Last     time.Time `json:"last"`
	Software string    `json:"software"`
	Version  string    `json:"version"`
	Filter   string    `json:"filter"`

	c *TCPAPRSClient // For closing

	Stats model.Statistics
}

// Clients records all clients
var Clients = make(map[any]*Client)

// InitListener inits listener daemon
func InitListener() {
	// Load init config
	load()

	// Add config change trigger
	config.OnChange = append(config.OnChange, func() {
		load()
	})

	logger.L.Debug("Listener initialized")
}

// load listener from config
func load() {
	// Close servers
	for _, v := range Listeners {
		v.s.Stop()
	}

	// Remove listeners
	Listeners = make(map[any]Listener)

	// Load config
	var listenersConfig []model.ListenerConfig
	marshalled, err := json.Marshal(config.C.Get("server.listeners"))
	if err != nil {
		logger.L.Error("Error loading listeners config", zap.Error(err))
		return
	}
	err = json.Unmarshal(marshalled, &listenersConfig)
	if err != nil {
		logger.L.Error("Error loading listeners config", zap.Error(err))
	}

	for _, listener := range listenersConfig {
		// TODO: Support more protocol
		if listener.Protocol != "tcp" {
			continue
		}

		// Create APRS server
		server := NewTCPAPRSServer(client.Mode(listener.Mode))

		// Record listener
		Listeners[server] = Listener{
			Name:     listener.Name,
			Type:     listener.Mode,
			Protocol: listener.Protocol,
			Host:     listener.Host,
			Port:     listener.Port,
			Visible:  listener.Visible,
			Filter:   listener.Filter,
			s:        server,
			Stats:    model.Statistics{},
		}

		// Start server
		err = server.Start(fmt.Sprintf("%s:%d", listener.Host, listener.Port))
		if err != nil {
			logger.L.Error("Error starting server", zap.Error(err))
		}
	}
}
