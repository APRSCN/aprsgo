package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/APRSCN/aprsgo/internal/handler"
	"github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/infra/cron"
	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"github.com/APRSCN/aprsgo/internal/network/listener"
	"github.com/APRSCN/aprsgo/internal/network/peer"
	"github.com/APRSCN/aprsgo/internal/network/uplink"
	"github.com/APRSCN/aprsgo/internal/system"
	"github.com/APRSCN/aprsgo/internal/upgrade"
	"github.com/gofiber/fiber/v3"
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

	// Init core peers
	peer.Init()

	// Apply configuration changes on SIGHUP: rebuild listeners, restart the
	// uplink manager and core peers so new settings take effect live.
	config.RegisterReloadHook(listener.Reload)
	config.RegisterReloadHook(uplink.Reload)
	config.RegisterReloadHook(peer.Reload)

	// Init cron
	cron.Init()

	// Run main server (serving the embedded web bundle)
	app := handler.Run(WebFS())

	// Setup signal handling for graceful shutdown and live upgrade.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	upgradeChan := make(chan os.Signal, 1)
	if sig := upgradeSignal(); sig != nil {
		signal.Notify(upgradeChan, sig)
	}

	// Main loop: handle a live-upgrade request by spawning a child that adopts
	// the listening sockets, then fall through to a graceful shutdown; any
	// terminating signal also leads to graceful shutdown.
	for {
		select {
		case sig := <-upgradeChan:
			logger.L.Info("received upgrade signal", zap.Any("signal", sig))
			if !upgrade.Supported() {
				logger.L.Warn("live upgrade not supported on this platform; ignoring")
				continue
			}
			pid, err := upgrade.Perform()
			if err != nil {
				logger.L.Error("live upgrade failed; continuing to run", zap.Error(err))
				continue
			}
			logger.L.Info("live upgrade: child started, draining and exiting",
				zap.Int("child_pid", pid))
			shutdown(app)
			return
		case sig := <-sigChan:
			logger.L.Info("received shutdown signal", zap.Any("signal", sig))
			shutdown(app)
			return
		}
	}
}

// shutdown stops the uplink managers and core peers, then gracefully shuts down
// the HTTP server within a bounded timeout.
func shutdown(app *fiber.App) {
	// Stop the uplink managers and active links.
	uplink.Stop()

	// Stop core peers.
	peer.Stop()

	// Graceful shutdown with 5 second timeout
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		logger.L.Error("error during graceful shutdown", zap.Error(err))
	}
}
