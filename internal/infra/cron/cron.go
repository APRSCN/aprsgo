package cron

import (
	"time"

	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"github.com/APRSCN/aprsgo/internal/network/listener"
	"github.com/APRSCN/aprsgo/internal/network/uplink"
	"github.com/APRSCN/aprsgo/internal/pkg/historydb"
	"github.com/go-co-op/gocron"
)

var C *gocron.Scheduler

// Init inits global cron object
func Init() {
	C = gocron.NewScheduler(time.Local)

	registerDefault()

	C.StartAsync()

	logger.L.Debug("cron initialized")
}

// registerDefault registers default cron tasks
func registerDefault() {
	// Periodically expire stale station positions used by range filters.
	if _, err := C.Every(30).Minutes().Do(func() {
		historydb.Positions.Cleanup()
	}); err != nil {
		logger.L.Error("failed to register position cleanup task")
	}

	// Periodically prune the long-lived duplicate checkers (shared submit
	// endpoint store and the uplink store) so expired keys do not accumulate.
	if _, err := C.Every(5).Minutes().Do(func() {
		listener.SweepSubmitDedup()
		uplink.SweepDupes()
	}); err != nil {
		logger.L.Error("failed to register dedup cleanup task")
	}
}
