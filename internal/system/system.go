package system

import (
	"time"

	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

var Status model.SystemStatus

// Daemon is the system daemon
func Daemon() {
	for {
		var cpuPercent, memTotal, memUsed float64
		for {
			percent, err := cpu.Percent(time.Second, false)
			if err != nil {
				continue
			}
			cpuPercent = percent[0]
			break
		}
		for {
			memInfo, err := mem.VirtualMemory()
			if err != nil {
				continue
			}
			memTotal = float64(memInfo.Total) / 1024 / 1024 / 1024
			memUsed = float64(memInfo.Used) / 1024 / 1024 / 1024
			break
		}
		Status = model.SystemStatus{
			Percent: cpuPercent,
			Total:   memTotal,
			Used:    memUsed,
		}
	}
}

// InitSystem inits system daemon
func InitSystem() {
	go Daemon()

	logger.L.Debug("System daemon initialized")
}
