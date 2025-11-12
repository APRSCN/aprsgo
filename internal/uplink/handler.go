package uplink

import (
	"time"

	"github.com/APRSCN/aprsgo/internal/historydb"
)

// packetHandler is the packet handler of uplink
func packetHandler(packet string) {
	// Get time now
	now := time.Now()

	// Write packet to stream
	Stream.Write(packet, "uplink")

	// Record last receive time
	err := historydb.C.Set([]byte("uplink.last"), []byte(time.Now().Format(time.RFC3339Nano)), 0)
	if err != nil {
		return
	}

	// Count packet rx
	err = historydb.IncrementValue("uplink.packet.rx.count", 1)
	if err != nil {
		return
	}

	// Write rx mark
	err = historydb.RecordDataPoint("uplink.packet.rx.speed", [2]any{
		float64(now.UnixNano()) / 1e9,
		nil,
	})
	if err != nil {
		return
	}
}

// sender sends data to uplink
func sender(dataCh <-chan StreamData) {
	for data := range dataCh {
		if data.Writer != "uplink" {
			// Get time now
			now := time.Now()

			// TODO: filter and dupecheck here
			_ = Client.SendPacket(data.Data)

			// Count packet tx
			err := historydb.IncrementValue("uplink.packet.tx.count", 1)
			if err != nil {
				return
			}

			// Write tx mark
			err = historydb.RecordDataPoint("uplink.packet.tx.speed", [2]any{
				float64(now.UnixNano()) / 1e9,
				nil,
			})
			if err != nil {
				return
			}
		}
	}
}
