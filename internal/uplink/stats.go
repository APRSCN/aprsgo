package uplink

import (
	"time"

	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/logger"
	"go.uber.org/zap"
)

// rate is the daemon to refresh rate
func rate() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Update rates
			currentSentPackets := Stats.SentPackets
			currentReceivedPackets := Stats.ReceivedPackets

			Stats.SendPacketRate = currentSentPackets - Stats.LastSentPackets
			Stats.RecvPacketRate = currentReceivedPackets - Stats.LastReceivedPackets

			Stats.LastSentPackets = currentSentPackets
			Stats.LastReceivedPackets = currentReceivedPackets
		}
	}
}

// stats is the daemon to record stats data
func stats() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Get time now
			now := time.Now()

			// Record packet rx rate
			err := historydb.RecordDataPoint("stats.uplink.packet.rx", [2]any{
				float64(now.UnixNano()) / 1e9,
				Stats.RecvPacketRate,
			})
			if err != nil {
				logger.L.Warn("Failed to record stats.uplink.packet.rx", zap.Error(err))
			}

			// Record bytes rx rate
			err = historydb.RecordDataPoint("stats.uplink.bytes.rx", [2]any{
				float64(now.UnixNano()) / 1e9,
				Client.GetStats().CurrentRecvRate,
			})
			if err != nil {
				logger.L.Warn("Failed to record stats.uplink.bytes.rx", zap.Error(err))
			}

			// Record packet tx rate
			err = historydb.RecordDataPoint("stats.uplink.packet.tx", [2]any{
				float64(now.UnixNano()) / 1e9,
				Stats.SendPacketRate,
			})
			if err != nil {
				logger.L.Warn("Failed to record stats.uplink.packet.tx", zap.Error(err))
			}

			// Record bytes tx rate
			err = historydb.RecordDataPoint("stats.uplink.bytes.tx", [2]any{
				float64(now.UnixNano()) / 1e9,
				Client.GetStats().CurrentSentRate,
			})
			if err != nil {
				logger.L.Warn("Failed to record stats.uplink.bytes.tx", zap.Error(err))
			}
		}
	}
}
