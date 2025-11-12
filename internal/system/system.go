package system

import (
	"os"
	"runtime"
	"time"

	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
	"go.uber.org/zap"
)

var Status model.SystemStatus

// Daemon is the system daemon
func Daemon() {
	lastHistoryDBRecord := time.Now().Add(-1 * time.Minute)

	for {
		// Get time now
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

		// Get system memory status
		for {
			memInfo, err := mem.VirtualMemory()
			if err != nil {
				continue
			}
			memTotal = float64(memInfo.Total) / 1024 / 1024
			memUsed = float64(memInfo.Used) / 1024 / 1024
			break
		}

		// Get self memory status
		p, err := process.NewProcess(int32(os.Getpid()))
		if err != nil {
			continue
		}
		memInfo, err := p.MemoryInfo()
		if err != nil {
			continue
		}
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Build the data
		Status = model.SystemStatus{
			Percent: cpuPercent,
			Memory: model.Memory{
				Total:             memTotal,
				Used:              memUsed,
				Self:              float64(memInfo.RSS) / 1024 / 1024,
				TotalAllocated:    float64(m.TotalAlloc) / 1024 / 1024,
				CurrentAllocated:  float64(m.Alloc) / 1024 / 1024,
				Malloc:            m.Mallocs,
				Free:              m.Frees,
				Heap:              float64(m.HeapAlloc) / 1024 / 1024,
				NumGC:             m.NumGC,
				PauseTotalSec:     float64(m.PauseTotalNs) / 1e9,
				LastGC:            time.UnixMicro(int64(m.LastGC / 1e3)),
				LastPauseTotalSec: float64(m.PauseNs[(m.NumGC+255)%256]) / 1e9,
				NextGC:            float64(m.NextGC) / 1024 / 1024,
				Lookups:           m.Lookups,
			},
		}

		if now.Sub(lastHistoryDBRecord) >= time.Minute {
			// Record new data
			for {
				err := historydb.RecordDataPoint("stats.memory", [2]any{
					float64(now.UnixNano()) / 1e9,
					Status.Memory.Self,
				})
				if err != nil {
					continue
				}
				lastHistoryDBRecord = now
				break
			}

			// Clear expired data
			err := historydb.ClearDataSlice("stats.memory", 30*24*60*60)
			if err != nil {
				logger.L.Warn("Failed to clear data points for stats.memory", zap.Error(err))
			}
		}
	}
}

// InitSystem inits system daemon
func InitSystem() {
	go Daemon()

	logger.L.Debug("System daemon initialized")
}
