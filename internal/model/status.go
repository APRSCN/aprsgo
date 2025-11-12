package model

import (
	"time"

	"github.com/APRSCN/aprsutils/client"
)

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
	Memory   Memory    `json:"memory"`
}

// ReturnUplink provides a struct to return uplink info
type ReturnUplink struct {
	ID            string          `json:"id"`
	Mode          client.Mode     `json:"mode"`
	Protocol      client.Protocol `json:"protocol"`
	Host          string          `json:"host"`
	Port          int             `json:"port"`
	Server        string          `json:"server"`
	Up            bool            `json:"up"`
	Uptime        time.Time       `json:"uptime"`
	Last          string          `json:"last"`
	PacketRX      uint64          `json:"packetRX"`
	PacketRXSpeed uint64          `json:"packetRXSpeed"`
	PacketTX      uint64          `json:"packetTX"`
	PacketTXSpeed uint64          `json:"packetTXSpeed"`
	BytesRX       uint64          `json:"bytesRX"`
	BytesRXSpeed  uint64          `json:"bytesRXSpeed"`
	BytesTX       uint64          `json:"bytesTX"`
	BytesTXSpeed  uint64          `json:"bytesTXSpeed"`
}

// ReturnListener provides a struct to return listener info
type ReturnListener struct {
	Name     string `json:"name"`
	Mode     string `json:"mode"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

// ReturnStatus provides a struct to return status of server
type ReturnStatus struct {
	Msg       string           `json:"msg"`
	Server    ReturnServer     `json:"server"`
	Uplink    ReturnUplink     `json:"uplink"`
	Listeners []ReturnListener `json:"listeners"`
}
