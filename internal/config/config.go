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

// staticConfig is constructor of static config
func staticConfig() *viper.Viper {
	// Init viper
	cfg := viper.New()

	// Set config type
	cfg.SetConfigType("yaml")

	// Set config path
	cfg.AddConfigPath("./")

	// Set config file
	cfg.SetConfigName("config")

	// Read the config file
	if err := cfg.ReadInConfig(); err != nil {
		log.Fatal("Failed to read config", err)
	}

	// Is debug mode?
	if _, err := os.Stat("config_debug.yaml"); err == nil {
		// Init config file
		cfg.SetConfigName("config_debug")

		// Set debug status
		Debug = true

		// Read the debug config file
		if err = cfg.ReadInConfig(); err != nil {
			log.Fatal("Failed to read debug config", err)
		}
	}

	// Unmarshal config
	if err := cfg.Unmarshal(&config); err != nil {
		log.Fatal("Failed to unmarshal config", err)
	}

	return cfg
}

// reload static config
func reload() {
	configRWMutex.Lock()
	defer configRWMutex.Unlock()

	staticConfig()
}

// Load static config
func Load() {
	configRWMutex.Lock()
	defer configRWMutex.Unlock()

	staticConfig()

	// Prepare a channel to receive signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP)

	// Start a goroutine to listen for signals
	go func() {
		for {
			sig := <-sigChan
			switch sig {
			case syscall.SIGHUP:
				reload()
			}
		}
	}()
}

// Get returns static config
func Get() StaticConfig {
	configRWMutex.RLock()
	defer configRWMutex.RUnlock()

	return config
}
