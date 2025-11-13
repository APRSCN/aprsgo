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
	ID           string          `json:"id"`
	Mode         client.Mode     `json:"mode"`
	Protocol     client.Protocol `json:"protocol"`
	Host         string          `json:"host"`
	Port         int             `json:"port"`
	Server       string          `json:"server"`
	Up           bool            `json:"up"`
	Uptime       time.Time       `json:"uptime"`
	Last         time.Time       `json:"last"`
	PacketRX     uint64          `json:"packetRX"`
	PacketRXRate uint64          `json:"packetRXRate"`
	PacketTX     uint64          `json:"packetTX"`
	PacketTXRate uint64          `json:"packetTXRate"`
	BytesRX      uint64          `json:"bytesRX"`
	BytesRXRate  uint64          `json:"bytesRXRate"`
	BytesTX      uint64          `json:"bytesTX"`
	BytesTXRate  uint64          `json:"bytesTXRate"`
}

// ReturnListener provides a struct to return listener info
type ReturnListener struct {
	Name         string `json:"name"`
	Mode         string `json:"mode"`
	Protocol     string `json:"protocol"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	PacketRX     uint64 `json:"packetRX"`
	PacketRXRate uint64 `json:"packetRXRate"`
	PacketTX     uint64 `json:"packetTX"`
	PacketTXRate uint64 `json:"packetTXRate"`
	BytesRX      uint64 `json:"bytesRX"`
	BytesRXRate  uint64 `json:"bytesRXRate"`
	BytesTX      uint64 `json:"bytesTX"`
	BytesTXRate  uint64 `json:"bytesTXRate"`
}

// ReturnClient provides a struct to return client info
type ReturnClient struct {
	At           string    `json:"at"`
	ID           string    `json:"id"`
	Addr         string    `json:"addr"`
	Uptime       time.Time `json:"uptime"`
	Last         time.Time `json:"last"`
	Software     string    `json:"software"`
	Version      string    `json:"version"`
	Filter       string    `json:"filter"`
	PacketRX     uint64    `json:"packetRX"`
	PacketRXRate uint64    `json:"packetRXRate"`
	PacketTX     uint64    `json:"packetTX"`
	PacketTXRate uint64    `json:"packetTXRate"`
	BytesRX      uint64    `json:"bytesRX"`
	BytesRXRate  uint64    `json:"bytesRXRate"`
	BytesTX      uint64    `json:"bytesTX"`
	BytesTXRate  uint64    `json:"bytesTXRate"`
}

// ReturnStatus provides a struct to return status of server
type ReturnStatus struct {
	Msg       string           `json:"msg"`
	Server    ReturnServer     `json:"server"`
	Uplink    ReturnUplink     `json:"uplink"`
	Listeners []ReturnListener `json:"listeners"`
	Clients   []ReturnClient   `json:"clients"`
}
