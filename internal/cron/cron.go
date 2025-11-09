package cron

import (
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/robfig/cron/v3"
)

var C *cron.Cron

// InitCron inits global cron object
func InitCron() {
	C = cron.New()

	registerDefault()

	C.Start()

	logger.L.Debug("Cron initialized")
}

// registerDefault registers default cron tasks
func registerDefault() {
}
