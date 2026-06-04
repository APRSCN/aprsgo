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

// ReturnUplink is uplink info. An uplink is a one-way link to an upstream
// server (this server is the child).
type ReturnUplink struct {
	ID           string          `json:"id"`
	Mode         client.Mode     `json:"mode"`
	Protocol     client.Protocol `json:"protocol"`
	Host         string          `json:"host"`      // configured hostname
	RealAddr     string          `json:"real_addr"` // resolved remote IP:port
	Port         int             `json:"port"`
	ServerID     string          `json:"server_id"` // upstream server callsign
	Server       string          `json:"server"`    // upstream software banner
	Up           bool            `json:"up"`
	Uptime       time.Time       `json:"uptime"`
	Last         time.Time       `json:"last"`
	PacketRX     uint64          `json:"packet_rx"`
	PacketRXDup  uint64          `json:"packet_rx_dup"`
	PacketRXErr  uint64          `json:"packet_rx_err"`
	PacketRXRate uint64          `json:"packet_rx_rate"`
	PacketTX     uint64          `json:"packet_tx"`
	PacketTXRate uint64          `json:"packet_tx_rate"`
	BytesRX      uint64          `json:"bytes_rx"`
	BytesRXRate  uint64          `json:"bytes_rx_rate"`
	BytesTX      uint64          `json:"bytes_tx"`
	BytesTXRate  uint64          `json:"bytes_tx_rate"`
}

// ReturnPeer is core-peer info. A peer is a symmetric two-way server link.
type ReturnPeer struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	Addr string `json:"addr"`
}

// ReturnListener provides a struct to return listener info
type ReturnListener struct {
	Name         string `json:"name"`
	Mode         string `json:"mode"`
	Protocol     string `json:"protocol"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Filter       string `json:"filter"`
	OnlineClient int    `json:"online_client"`
	PeakClient   int    `json:"peak_client"`
	PacketRX     uint64 `json:"packet_rx"`
	PacketRXRate uint64 `json:"packet_rx_rate"`
	PacketTX     uint64 `json:"packet_tx"`
	PacketTXRate uint64 `json:"packet_tx_rate"`
	BytesRX      uint64 `json:"bytes_rx"`
	BytesRXRate  uint64 `json:"bytes_rx_rate"`
	BytesTX      uint64 `json:"bytes_tx"`
	BytesTXRate  uint64 `json:"bytes_tx_rate"`
}

// ReturnClient provides a struct to return client info
type ReturnClient struct {
	At           string    `json:"at"`
	Port         int       `json:"port"`
	ID           string    `json:"id"`
	Verified     bool      `json:"verified"`
	Addr         string    `json:"addr"`
	Uptime       time.Time `json:"uptime"`
	Last         time.Time `json:"last"`
	Software     string    `json:"software"`
	Version      string    `json:"version"`
	Filter       string    `json:"filter"`
	OutQ         int       `json:"out_q"`     // bytes queued for delivery
	MsgRcpts     int       `json:"msg_rcpts"` // distinct stations heard
	PacketRX     uint64    `json:"packet_rx"`
	PacketRXDup  uint64    `json:"packet_rx_dup"`
	PacketRXErr  uint64    `json:"packet_rx_err"`
	PacketRXRate uint64    `json:"packet_rx_rate"`
	PacketTX     uint64    `json:"packet_tx"`
	PacketTXRate uint64    `json:"packet_tx_rate"`
	BytesRX      uint64    `json:"bytes_rx"`
	BytesRXRate  uint64    `json:"bytes_rx_rate"`
	BytesTX      uint64    `json:"bytes_tx"`
	BytesTXRate  uint64    `json:"bytes_tx_rate"`
}

// ReturnTotals aggregates process-wide counters for the status page.
type ReturnTotals struct {
	// Connected client count and process uptime context.
	Clients int `json:"clients"`

	// Cumulative packet/byte counters across all inbound ports.
	PacketRX uint64 `json:"packet_rx"`
	PacketTX uint64 `json:"packet_tx"`
	BytesRX  uint64 `json:"bytes_rx"`
	BytesTX  uint64 `json:"bytes_tx"`

	// Per-second rates.
	PacketRXRate uint64 `json:"packet_rx_rate"`
	PacketTXRate uint64 `json:"packet_tx_rate"`
	BytesRXRate  uint64 `json:"bytes_rx_rate"`
	BytesTXRate  uint64 `json:"bytes_tx_rate"`

	// Duplicate packets dropped.
	Dupes uint64 `json:"dupes"`

	// Station position-cache and message-routing sizes.
	PositionCache int `json:"position_cache"`
}

// ReturnStatus provides a struct to return status of server
type ReturnStatus struct {
	Msg       string            `json:"msg"`
	Server    ReturnServer      `json:"server"`
	Totals    ReturnTotals      `json:"totals"`
	Uplink    *ReturnUplink     `json:"uplink"`
	Peers     []*ReturnPeer     `json:"peers"`
	Listeners []*ReturnListener `json:"listeners"`
	Clients   []*ReturnClient   `json:"clients"`
}
