package uplink

import (
	"hash/fnv"
	"time"

	"github.com/APRSCN/aprsgo/internal/historydb"
)

var dupRecords *historydb.MapFloat64History

// recvHandler is the packet handler of uplink
func recvHandler(packet string) {
	// Get time now
	now := time.Now()

	// Hash the data (dup check)
	h64 := fnv.New64a()
	_, err := h64.Write([]byte(packet))
	if err == nil {
		hash64 := h64.Sum64()

		if dupRecords.Contain(hash64) {
			return
		}

		go dupRecords.Record(hash64, float64(now.UnixNano())/1e9)
		go dupRecords.ClearByValue(1)
	}

	// Write packet to stream
	Stream.Write(packet, "uplink")

	// Record last receive time
	Last = now

	// Count packet rx
	Stats.ReceivedPackets++
}

// sendHandler sends data to uplink
func sendHandler(dataCh <-chan StreamData) {
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
