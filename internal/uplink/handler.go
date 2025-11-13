package uplink

import (
	"time"
)

// packetHandler is the packet handler of uplink
func packetHandler(packet string) {
	// Get time now
	now := time.Now()

	// Write packet to stream
	Stream.Write(packet, "uplink")

	// Record last receive time
	Last = now

	// Count packet rx
	Stats.ReceivedPackets++
}

// sender sends data to uplink
func sender(dataCh <-chan StreamData) {
	for data := range dataCh {
		if data.Writer != "uplink" {
			// Get time now
			//now := time.Now()

			// TODO: filter and dupecheck here
			_ = Client.SendPacket(data.Data)

			// Count packet tx
			Stats.SentPackets++
		}
	}
}
