package cron

import (
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
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
	_, err := C.AddFunc("@every 1s", ClearSpeed)
	if err != nil {
		logger.L.Fatal("Failed to register ClearSpeed cron", zap.Error(err))
	}
}
