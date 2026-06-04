package model

import (
	"reflect"

	"go.gh.ink/json"
)

// Statistics holds all statistics data
type Statistics struct {
	SentPackets     uint64 `json:"sent_packets"`
	ReceivedPackets uint64 `json:"received_packets"`
	ReceivedDups    uint64 `json:"received_dups"`
	ReceivedErrors  uint64 `json:"received_errors"`
	ReceivedQDrop   uint64 `json:"received_q_drop"`
	SentBytes       uint64 `json:"sent_bytes"`
	ReceivedBytes   uint64 `json:"received_bytes"`

	// Rates (packets per second)
	SendPacketRate uint64 `json:"send_packet_rate"`
	RecvPacketRate uint64 `json:"recv_packet_rate"`
	SendByteRate   uint64 `json:"send_byte_rate"`
	RecvByteRate   uint64 `json:"recv_byte_rate"`
}

// StatsReturn provides a struct to return stats of server
type StatsReturn struct {
	Msg            string   `json:"msg"`
	Memory         [][2]any `json:"memory"`
	UplinkPacketRX [][2]any `json:"uplink_packet_rx"`
	UplinkPacketTX [][2]any `json:"uplink_packet_tx"`
	UplinkBytesRX  [][2]any `json:"uplink_bytes_rx"`
	UplinkBytesTX  [][2]any `json:"uplink_bytes_tx"`
}

func init() {
	_ = json.PreheatMany([]reflect.Type{
		reflect.TypeOf(Statistics{}),
		reflect.TypeOf(StatsReturn{}),
	})
}
