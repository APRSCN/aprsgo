package model

// Statistics holds all statistics data
type Statistics struct {
	// Client statistics
	SentPackets     uint64 `json:"sentPackets"`     // Total packets sent to client
	ReceivedPackets uint64 `json:"receivedPackets"` // Total packets received from client
	SentBytes       uint64 `json:"sentBytes"`       // Total bytes sent to client
	ReceivedBytes   uint64 `json:"receivedBytes"`   // Total bytes received from client

	// Rates (packets per second)
	SendPacketRate uint64 `json:"sendPacketRate"` // Current send packet rate
	RecvPacketRate uint64 `json:"recvPacketRate"` // Current receive packet rate
	SendByteRate   uint64 `json:"sendByteRate"`   // Current send byte rate
	RecvByteRate   uint64 `json:"recvByteRate"`   // Current receive byte rate

	// Internal counters for rate calculation
	LastSentPackets     uint64
	LastReceivedPackets uint64
	LastSentBytes       uint64
	LastReceivedBytes   uint64
}

// StatsReturn provides a struct to return stats of server
type StatsReturn struct {
	Msg            string   `json:"msg"`
	Memory         [][2]any `json:"memory"`
	UplinkPacketRX [][2]any `json:"uplinkPacketRX"`
	UplinkPacketTX [][2]any `json:"uplinkPacketTX"`
	UplinkBytesRX  [][2]any `json:"uplinkBytesRX"`
	UplinkBytesTX  [][2]any `json:"uplinkBytesTX"`
}
