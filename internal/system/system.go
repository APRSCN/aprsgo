package system

import (
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/internal/pkg/historydb"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

// status holds the latest system status behind an atomic pointer so the
// daemon can publish a fresh snapshot while the status HTTP handler reads it
// concurrently without a data race.
var status atomic.Pointer[model.SystemStatus]

// Snapshot returns the latest system status (zero value until the daemon has
// published its first sample).
func Snapshot() model.SystemStatus {
	if p := status.Load(); p != nil {
		return *p
	}
	return model.SystemStatus{}
}

var StatsMemory *historydb.MapFloat64History

// Daemon is the system daemon
func Daemon() {
	lastHistoryDBRecord := time.Now().Add(-1 * time.Minute)

	for {
		// Get time now
		now := time.Now()
		var cpuPercent, memTotal, memUsed float64

		// Get CPU per cent (cpu.Percent blocks for the sample interval, so this
		// loop is naturally paced; back off on error to avoid a tight spin).
		for {
			percent, err := cpu.Percent(time.Second, false)
			if err != nil || len(percent) == 0 {
				time.Sleep(time.Second)
				continue
			}
			cpuPercent = percent[0]
			break
		}

		// Get system memory status
		for {
			memInfo, err := mem.VirtualMemory()
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			memTotal = float64(memInfo.Total) / 1024 / 1024
			memUsed = float64(memInfo.Used) / 1024 / 1024
			break
		}

		// Get self memory status
		p, err := process.NewProcess(int32(os.Getpid()))
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		memInfo, err := p.MemoryInfo()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Build and publish the snapshot atomically.
		snap := model.SystemStatus{
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
		status.Store(&snap)

		if now.Sub(lastHistoryDBRecord) >= time.Minute {
			// Record new data
			StatsMemory.Record(float64(now.UnixNano())/1e9, snap.Memory.Self)

			// ClearByValue expired data
			StatsMemory.ClearByKey(30 * 24 * 60 * 60)

			lastHistoryDBRecord = time.Now()
		}
	}
}

// Init inits system daemon
func Init() {
	// Init stats
	StatsMemory = historydb.NewMapFloat64History()

	// Start daemon
	go Daemon()

	logger.L.Debug("System daemon initialized")
}
