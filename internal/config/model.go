package config

type StaticConfig struct {
	Server struct {
		// Buff size setting (KB)
		BuffSize int `yaml:"buffSize"`
		// Custom processor name
		// Only available when we can not get CPU info
		Model string `yaml:"model"`
		// Your server ID
		ID string `yaml:"id"`
		// Passcode for the server ID
		Passcode string `yaml:"passcode"`
		// Setting of http status panel
		Status struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		} `yaml:"status"`
		// Setting of aprs server
		// Mode: fullfeed [Everything] / igate [IGate / Client Port]
		Listeners []struct {
			Name     string `yaml:"name"`
			Mode     string `yaml:"mode"`
			Protocol string `yaml:"protocol"`
			Host     string `yaml:"host"`
			Port     int    `yaml:"port"`
			Visible  string `yaml:"visible"`
			Filter   string `yaml:"filter"`
		} `yaml:"listeners"`
		Uplinks []struct {
			Name     string `yaml:"name"`
			Mode     string `yaml:"mode"`
			Protocol string `yaml:"protocol"`
			Host     string `yaml:"host"`
			Port     int    `yaml:"port"`
		} `yaml:"uplinks"`
	} `yaml:"server"`
	// Info of server admin
	Admin struct {
		Name  string `yaml:"name"`
		Email string `yaml:"email"`
	} `yaml:"admin"`
	// Config of system log
	Log struct {
		File       string `yaml:"file"`
		MaxSize    int    `yaml:"maxSize"`
		MaxBackups int    `yaml:"maxBackups"`
		MaxAge     int    `yaml:"maxAge"`
		Compress   bool   `yaml:"compress"`
	} `yaml:"log"`
}
