package main

import (
	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/cron"
	"github.com/APRSCN/aprsgo/internal/handler"
	"github.com/APRSCN/aprsgo/internal/listener"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/system"
	"github.com/APRSCN/aprsgo/internal/uplink"

	"go.uber.org/zap"
)

func main() {
	// Init embed
	InitEmbed()

	// Load static config
	config.LoadStatic()

	// Init logger
	logger.InitLogger()
	defer func(L *zap.Logger) {
		_ = L.Sync()
	}(logger.L)

	// Init system daemon
	system.InitSystem()

	// Init uplink
	uplink.InitUplink()

	// Init listener
	listener.InitListener()

	// Init cron
	cron.InitCron()

	// Start HTTP server
	handler.RunHTTPServer()

	select {}
}
