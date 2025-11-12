package historydb

import (
	"runtime/debug"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/coocood/freecache"
)

var C *freecache.Cache

// InitHistoryDB inits history DB
func InitHistoryDB() {
	// Calc memory size
	cacheSize := config.C.GetInt("server.memory") * 1024 * 1024
	// Create cache object
	C = freecache.NewCache(cacheSize)
	debug.SetGCPercent(20)

	logger.L.Debug("HistoryDB initialized")
}
