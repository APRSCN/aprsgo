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

type Listener struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Visible  string `json:"visible"`
	Filter   string `json:"filter"`
}

var Listeners = make([]Listener, 0)

type Client struct {
	At       string    `json:"at"`
	ID       string    `json:"id"`
	Addr     string    `json:"addr"`
	Uptime   time.Time `json:"uptime"`
	Last     time.Time `json:"last"`
	Software string    `json:"software"`
	Version  string    `json:"version"`
	Filter   string    `json:"filter"`
	c        *TCPAPRSClient
}

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
	// Remove listeners
	Listeners = make([]Listener, 0)

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
		Listeners = append(Listeners, Listener{
			Name:     listener.Name,
			Type:     listener.Mode,
			Protocol: listener.Protocol,
			Host:     listener.Host,
			Port:     listener.Port,
			Visible:  listener.Visible,
			Filter:   listener.Filter,
		})
		go func() {
			// Create APRS server
			server := NewTCPAPRSServer(client.Mode(listener.Mode))

			// Start server
			err = server.Start(fmt.Sprintf("%s:%d", listener.Host, listener.Port))
			if err != nil {
				logger.L.Error("Error starting server", zap.Error(err))
			}
		}()
	}
}
