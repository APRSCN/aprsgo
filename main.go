package main

import (
	"log"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/cron"
	"github.com/APRSCN/aprsgo/internal/handler"
	"github.com/APRSCN/aprsgo/internal/listener"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/system"
	"github.com/APRSCN/aprsgo/internal/uplink"

	"net/http"
	_ "net/http/pprof" // 导入 pprof，自动注册路由

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
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	select {}
}
