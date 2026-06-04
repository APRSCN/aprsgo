package config

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/spf13/viper"
)

var config StaticConfig
var configRWMutex sync.RWMutex

// debug holds the debug-mode flag. It is written while loading the
// configuration (including on SIGHUP reload) and read concurrently by other
// subsystems, so it is atomic. Read it via Debug().
var debug atomic.Bool

var signalStopChan chan struct{}

// Debug reports whether debug mode is enabled (presence of config_debug.yaml).
func Debug() bool { return debug.Load() }

// reloadHooks are invoked (in registration order) after the configuration is
// successfully re-read on SIGHUP, so subsystems can apply the new settings.
var (
	reloadHooks   []func()
	reloadHooksMu sync.Mutex
)

// RegisterReloadHook registers a callback to run after each successful SIGHUP
// reload. Hooks must be safe to call from the signal-handling goroutine.
func RegisterReloadHook(fn func()) {
	reloadHooksMu.Lock()
	reloadHooks = append(reloadHooks, fn)
	reloadHooksMu.Unlock()
}

// runReloadHooks invokes all registered reload hooks.
func runReloadHooks() {
	reloadHooksMu.Lock()
	hooks := make([]func(), len(reloadHooks))
	copy(hooks, reloadHooks)
	reloadHooksMu.Unlock()
	for _, fn := range hooks {
		fn()
	}
}

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

		// Read the debug config file
		if err = cfg.ReadInConfig(); err != nil {
			return nil, err
		}

		// Set debug status only after the debug config has been read
		// successfully, so a failed read does not leave the flag set.
		debug.Store(true)
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
	_, err := load()
	configRWMutex.Unlock()

	if err != nil {
		log.Println("reload static config failed:", err)
		return
	}

	// Apply the new configuration to subsystems (listeners, uplinks, peers).
	// Run outside the config lock so hooks may call Get().
	runReloadHooks()
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

// Set replaces the in-memory static config. It is primarily intended for tests
// and programmatic configuration; normal startup loads from file via Init.
func Set(c StaticConfig) {
	configRWMutex.Lock()
	defer configRWMutex.Unlock()
	config = c
}
