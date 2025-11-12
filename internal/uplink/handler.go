package uplink

import (
	"bytes"
	"time"

	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/logger"
	"go.uber.org/zap"
)

var Packets bytes.Buffer

// packetHandler is the packet handler of uplink
func packetHandler(packet string) {
	// Get time now
	now := time.Now()

	// Write packet to stream
	Stream.Write(packet)

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

	err = historydb.RecordDataPoint("uplink.packet.rx.speed", [2]any{
		float64(now.UnixNano()) / 1e9,
		nil,
	})
	if err != nil {
		return
	}
	// Clear expired data
	err = historydb.ClearDataSlice("uplink.packet.rx.speed", 1)
	if err != nil {
		logger.L.Warn("Failed to clear data points for uplink.packet.rx.speed", zap.Error(err))
		return
	}
}
