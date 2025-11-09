package config

import (
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var C *viper.Viper
var OnChange []func()
var lastLoad time.Time

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
	err := cfg.ReadInConfig()
	if err != nil {
		panic(err)
	}

	if cfg.GetBool("debug") {
		// Init config file
		cfg.SetConfigName("config_debug")

		// Read the debug config file
		err = cfg.ReadInConfig()
		if err != nil {
			panic(err)
		}
	}

	// Watch config change
	cfg.WatchConfig()

	// Record first load
	lastLoad = time.Now()

	// Trigger to reload
	cfg.OnConfigChange(func(e fsnotify.Event) {
		// Debounce
		if lastLoad.Add(time.Second * 1).After(time.Now()) {
			return
		}

		for _, fn := range OnChange {
			fn()
		}

		lastLoad = time.Now()
	})

	return cfg
}

// LoadStatic loads static config
func LoadStatic() *viper.Viper {
	C = staticConfig()
	return C
}
