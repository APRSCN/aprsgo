package cron

import (
	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/logger"
	"go.uber.org/zap"
)

// ClearRate clears expired rate record
func ClearRate() {
	// Clear expired tx record
	err := historydb.ClearDataSlice("uplink.packet.tx.rate", 1)
	if err != nil {
		logger.L.Warn("Failed to clear data points for uplink.packet.tx.rate", zap.Error(err))
	}

	// Clear expired rx record
	err = historydb.ClearDataSlice("uplink.packet.rx.rate", 1)
	if err != nil {
		logger.L.Warn("Failed to clear data points for uplink.packet.rx.rate", zap.Error(err))
	}
}
