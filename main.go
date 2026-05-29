package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// Load public config
	config.Init()
	defer config.Cleanup()

	// Init logger
	logger.Init()
	defer logger.Cleanup()

	// Init system daemon
	system.Init()

	// Init uplink
	uplink.Init()

	// Init listener
	listener.Init()

	// Init cron
	cron.Init()

	// Run main server
	app := handler.Run()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	sig := <-sigChan
	logger.L.Info("received shutdown signal", zap.Any("signal", sig))

	// Graceful shutdown with 5 second timeout
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		logger.L.Error("error during graceful shutdown", zap.Error(err))
	}
}
