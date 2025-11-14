package model

// Statistics holds all statistics data
type Statistics struct {
	SentPackets     uint64 `json:"sentPackets"`
	ReceivedPackets uint64 `json:"receivedPackets"`
	ReceivedDups    uint64 `json:"receivedDups"`
	ReceivedErrors  uint64 `json:"receivedErrors"`
	SentBytes       uint64 `json:"sentBytes"`
	ReceivedBytes   uint64 `json:"receivedBytes"`

	// Rates (packets per second)
	SendPacketRate uint64 `json:"sendPacketRate"`
	RecvPacketRate uint64 `json:"recvPacketRate"`
	SendByteRate   uint64 `json:"sendByteRate"`
	RecvByteRate   uint64 `json:"recvByteRate"`

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
