package main

import (
	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/cron"
	"github.com/APRSCN/aprsgo/internal/handler"
	"github.com/APRSCN/aprsgo/internal/logger"

	"go.uber.org/zap"
)

func main() {
	// Load static config
	config.LoadStatic()

	// Init logger
	logger.InitLogger()
	defer func(L *zap.Logger) {
		_ = L.Sync()
	}(logger.L)

	// Init cron
	cron.InitCron()

	// Start HTTP server
	handler.RunHTTPServer()

	select {}
}
