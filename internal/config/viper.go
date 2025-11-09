package config

import (
	"github.com/spf13/viper"
)

var C *viper.Viper

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

	return cfg
}

// LoadStatic loads static config
func LoadStatic() *viper.Viper {
	C = staticConfig()
	return C
}
