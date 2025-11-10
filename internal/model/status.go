package model

import "time"

// ReturnServer provides a struct to return basic server info
type ReturnServer struct {
	Admin    string    `json:"admin"`
	Email    string    `json:"email"`
	OS       string    `json:"os"`
	Arch     string    `json:"arch"`
	ID       string    `json:"id"`
	Software string    `json:"software"`
	Version  string    `json:"version"`
	Now      time.Time `json:"now"`
	Uptime   float64   `json:"uptime"`
	Model    string    `json:"model"`
	Percent  float64   `json:"percent"`
	Total    float64   `json:"total"`
	Used     float64   `json:"used"`
}

// ReturnListener provides a struct to return listener info
type ReturnListener struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

// ReturnStatus provides a struct to return status of server
type ReturnStatus struct {
	Msg       string           `json:"msg"`
	Server    ReturnServer     `json:"server"`
	Listeners []ReturnListener `json:"listeners"`
}
