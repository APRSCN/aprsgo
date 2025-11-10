package system

import (
	"time"

	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"go.uber.org/zap"
)

var Status model.SystemStatus

// Daemon is the system daemon
func Daemon() {
	lastHistoryDBRecord := time.Now().Add(-1 * time.Minute)

	for {
		now := time.Now()
		var cpuPercent, memTotal, memUsed float64
		// Get CPU percent
		for {
			percent, err := cpu.Percent(time.Second, false)
			if err != nil {
				continue
			}
			cpuPercent = percent[0]
			break
		}
		// Get memory status
		for {
			memInfo, err := mem.VirtualMemory()
			if err != nil {
				continue
			}
			memTotal = float64(memInfo.Total) / 1024 / 1024 / 1024
			memUsed = float64(memInfo.Used) / 1024 / 1024 / 1024
			break
		}
		// Build the data
		Status = model.SystemStatus{
			Percent: cpuPercent,
			Total:   memTotal,
			Used:    memUsed,
		}

		if now.Sub(lastHistoryDBRecord) >= time.Minute {
			// Record new data
			for {
				err := historydb.RecordDataPoint("status.system", historydb.DataPoint{
					Timestamp: now,
					Value:     Status,
				})
				if err != nil {
					continue
				}
				lastHistoryDBRecord = now
				break
			}

			// Clear expired data
			err := historydb.ClearDataSlice("status.system", 24*30*time.Hour)
			if err != nil {
				logger.L.Warn("Failed to clear data points for status.system", zap.Error(err))
			}
		}
	}
}

// InitSystem inits system daemon
func InitSystem() {
	go Daemon()

	logger.L.Debug("System daemon initialized")
}
