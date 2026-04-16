package config

type StaticConfig struct {
	Server struct {
		// Buff size setting (KB)
		BuffSize int `mapstructure:"buffSize"`
		// Custom processor name
		// Only available when we can not get CPU info
		Model string `mapstructure:"model"`
		// Your server ID
		ID string `mapstructure:"id"`
		// Passcode for the server ID
		Passcode string `mapstructure:"passcode"`
		// Setting of http status panel
		Status struct {
			Host string `mapstructure:"host"`
			Port int    `mapstructure:"port"`
		} `mapstructure:"status"`
		// Setting of aprs server
		// Mode: fullfeed [Everything] / igate [IGate / Client Port]
		Listeners []struct {
			Name     string `mapstructure:"name"`
			Mode     string `mapstructure:"mode"`
			Protocol string `mapstructure:"protocol"`
			Host     string `mapstructure:"host"`
			Port     int    `mapstructure:"port"`
			Visible  string `mapstructure:"visible"`
			Filter   string `mapstructure:"filter"`
		} `mapstructure:"listeners"`
		Uplinks []struct {
			Name     string `mapstructure:"name"`
			Mode     string `mapstructure:"mode"`
			Protocol string `mapstructure:"protocol"`
			Host     string `mapstructure:"host"`
			Port     int    `mapstructure:"port"`
		} `mapstructure:"uplinks"`
	} `mapstructure:"server"`
	// Info of server admin
	Admin struct {
		Name  string `mapstructure:"name"`
		Email string `mapstructure:"email"`
	} `mapstructure:"admin"`
	// Config of system log
	Log struct {
		File       string `mapstructure:"file"`
		MaxSize    int    `mapstructure:"maxSize"`
		MaxBackups int    `mapstructure:"maxBackups"`
		MaxAge     int    `mapstructure:"maxAge"`
		Compress   bool   `mapstructure:"compress"`
	} `mapstructure:"log"`
}
