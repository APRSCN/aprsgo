package listener

// Statistics holds all statistics data for a client or server
type Statistics struct {
	// Client statistics
	SentPackets     uint64 `json:"sent_packets"`     // Total packets sent to client
	ReceivedPackets uint64 `json:"received_packets"` // Total packets received from client
	SentBytes       uint64 `json:"sent_bytes"`       // Total bytes sent to client
	ReceivedBytes   uint64 `json:"received_bytes"`   // Total bytes received from client

	// Rates (packets per second)
	SendPacketRate uint64 `json:"send_packet_rate"` // Current send packet rate
	RecvPacketRate uint64 `json:"recv_packet_rate"` // Current receive packet rate
	SendByteRate   uint64 `json:"send_byte_rate"`   // Current send byte rate
	RecvByteRate   uint64 `json:"recv_byte_rate"`   // Current receive byte rate

	// Internal counters for rate calculation
	lastSentPackets     uint64
	lastReceivedPackets uint64
	lastSentBytes       uint64
	lastReceivedBytes   uint64
}
