package model

// ListenerConfig provides a struct to load listeners from config
type ListenerConfig struct {
	Name     string `json:"name" yaml:"name"`
	Type     string `json:"type" yaml:"Type"`
	Protocol string `json:"protocol" yaml:"protocol"`
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Visible  string `json:"visible" yaml:"visible"`
	Filter   string `json:"filter" yaml:"filter"`
}
