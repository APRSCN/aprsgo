package cron

import (
	"time"

	"github.com/APRSCN/aprsgo/internal/logger"
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
}
