package cron

import (
	"github.com/robfig/cron/v3"
)

var C *cron.Cron

// InitCron inits global cron object
func InitCron() {
	C = cron.New()

	registerDefault()

	C.Start()
}

// registerDefault registers default cron tasks
func registerDefault() {
}
