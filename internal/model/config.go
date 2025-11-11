package model

import "github.com/APRSCN/aprsutils/client"

// ListenerConfig provides a struct to load listeners from config
type ListenerConfig struct {
	Name     string `json:"name" yaml:"name"`
	Mode     string `json:"mode" yaml:"mode"`
	Protocol string `json:"protocol" yaml:"protocol"`
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Visible  string `json:"visible" yaml:"visible"`
	Filter   string `json:"filter" yaml:"filter"`
}

// UplinkConfig provides a struct to load uplinks from config
type UplinkConfig struct {
	Name     string          `json:"name" yaml:"name"`
	Mode     client.Mode     `json:"mode" yaml:"mode"`
	Protocol client.Protocol `json:"protocol" yaml:"protocol"`
	Host     string          `json:"host" yaml:"host"`
	Port     int             `json:"port" yaml:"port"`
}
