package main

import (
	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/cron"
	"github.com/APRSCN/aprsgo/internal/handler"
	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/listener"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/system"

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

	// Init history DB
	historydb.InitHistoryDB()

	// Init system daemon
	system.InitSystem()

	// Init listener
	listener.InitListener()

	// Init cron
	cron.InitCron()

	// Start HTTP server
	handler.RunHTTPServer()

	select {}
}
