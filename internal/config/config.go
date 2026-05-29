package config

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/viper"
)

var config StaticConfig
var configRWMutex sync.RWMutex
var Debug = false
var signalStopChan chan struct{}

// load is constructor of static config
func load() (*viper.Viper, error) {
	// Init static config
	cfg := viper.New()

	// Set config type
	cfg.SetConfigType("yaml")

	// Set config path
	cfg.AddConfigPath("./")

	// Set config file
	cfg.SetConfigName("config")

	// Read the config file
	if err := cfg.ReadInConfig(); err != nil {
		return nil, err
	}

	// Is debug mode?
	if _, err := os.Stat("config_debug.yaml"); err == nil {
		// Init config file
		cfg.SetConfigName("config_debug")

		// Set debug status
		Debug = true

		// Read the debug config file
		if err = cfg.ReadInConfig(); err != nil {
			return nil, err
		}
	}

	// Unmarshal config
	if err := cfg.Unmarshal(&config); err != nil {
		return nil, err
	}

	return cfg, nil
}

// reload static config
func reload() {
	configRWMutex.Lock()
	defer configRWMutex.Unlock()

	if _, err := load(); err != nil {
		log.Println("reload static config failed:", err)
	}
}

// Init static config
func Init() {
	configRWMutex.Lock()
	defer configRWMutex.Unlock()

	if _, err := load(); err != nil {
		log.Fatal("load static config failed:", err)
	}

	// Prepare a channel to receive signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP)

	// Create stop channel
	signalStopChan = make(chan struct{})

	// Start a goroutine to listen for signals
	go func() {
		for {
			select {
			case sig := <-sigChan:
				if sig == syscall.SIGHUP {
					reload()
				}
			case <-signalStopChan:
				signal.Stop(sigChan)
				return
			}
		}
	}()
}

// Cleanup stops the signal listening goroutine
func Cleanup() {
	if signalStopChan != nil {
		close(signalStopChan)
	}
}

// Get returns static config
func Get() StaticConfig {
	configRWMutex.RLock()
	defer configRWMutex.RUnlock()

	return config
}
