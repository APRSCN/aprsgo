package listener

import (
	"encoding/json"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/model"
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

// InitListener inits listener daemon
func InitListener() {
	// Load init config
	load()

	// Add config change trigger
	config.OnChange = append(config.OnChange, func() {
		load()
	})

	logger.L.Debug("ReturnListener initialized")
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
		Listeners = append(Listeners, Listener{
			Name:     listener.Name,
			Type:     listener.Type,
			Protocol: listener.Protocol,
			Host:     listener.Host,
			Port:     listener.Port,
			Visible:  listener.Visible,
			Filter:   listener.Filter,
		})
	}
}
