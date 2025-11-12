package cron

import (
	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/logger"
	"go.uber.org/zap"
)

// ClearSpeed clears expired speed record
func ClearSpeed() {
	// Clear expired tx record
	err := historydb.ClearDataSlice("uplink.packet.tx.speed", 1)
	if err != nil {
		logger.L.Warn("Failed to clear data points for uplink.packet.tx.speed", zap.Error(err))
	}

	// Clear expired rx record
	err = historydb.ClearDataSlice("uplink.packet.rx.speed", 1)
	if err != nil {
		logger.L.Warn("Failed to clear data points for uplink.packet.rx.speed", zap.Error(err))
	}
}
