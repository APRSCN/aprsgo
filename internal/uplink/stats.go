package uplink

import (
	"time"

	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/logger"
	"go.uber.org/zap"
)

// stats is the daemon to record stats data
func stats() {
	// Waiting for uplink start
	time.Sleep(5 * time.Second)
	for {
		// Get time now
		now := time.Now()

		// Record packet rx speed
		rxRecent, err := historydb.GetDataSlice("uplink.packet.rx.speed")
		if err != nil {
			logger.L.Warn("Failed to read uplink.packet.rx.speed", zap.Error(err))
		} else {
			err = historydb.RecordDataPoint("stats.uplink.packet.rx", [2]any{
				float64(now.UnixNano()) / 1e9,
				len(rxRecent),
			})
			if err != nil {
				logger.L.Warn("Failed to record stats.uplink.packet.rx", zap.Error(err))
			}
		}

		// Record bytes rx speed
		err = historydb.RecordDataPoint("stats.uplink.bytes.rx", [2]any{
			float64(now.UnixNano()) / 1e9,
			Client.GetStats().CurrentRecvRate,
		})
		if err != nil {
			logger.L.Warn("Failed to record stats.uplink.bytes.rx", zap.Error(err))
		}

		// TODO: Add packet tx speed stats

		// Record bytes tx speed
		err = historydb.RecordDataPoint("stats.uplink.bytes.tx", [2]any{
			float64(now.UnixNano()) / 1e9,
			Client.GetStats().CurrentSentRate,
		})
		if err != nil {
			logger.L.Warn("Failed to record stats.uplink.bytes.tx", zap.Error(err))
		}

		time.Sleep(60 * time.Second)
	}
}
