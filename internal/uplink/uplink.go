package uplink

import (
	"fmt"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/meta"
	"github.com/APRSCN/aprsutils/client"
)

var Client *client.Client
var stop = false

// InitUplink inits uplink daemon
func InitUplink() {
	// Init Stream
	Stream = NewDataStream(100)

	// Init dupRecords
	dupRecords = historydb.NewMapFloat64History()

	// Init stats
	StatsPacketRX = historydb.NewMapFloat64History()
	StatsPacketTX = historydb.NewMapFloat64History()
	StatsBytesRX = historydb.NewMapFloat64History()
	StatsBytesTX = historydb.NewMapFloat64History()

	// Start uplink
	go selectUplink()

	// Start stats
	go rate()
	go stats()

	logger.L.Debug("Uplink daemon initialized")
}

// selectUplink is the daemon of uplink that will automatically choose an available uplink
func selectUplink() {
	// Reset flag
	stop = false

	defer Client.Close()
	// Select available node with fallthrough
	for !stop {
		for _, uplink := range config.Get().Server.Uplinks {
			// Create client
			Client = client.NewClient(
				config.Get().Server.ID,
				config.Get().Server.Passcode,
				client.Mode(uplink.Mode), client.Protocol(uplink.Protocol),
				uplink.Host, uplink.Port,
				client.WithBufSize(config.Get().Server.BuffSize*1024),
				client.WithLogger(&ZapLogger{logger: logger.L}),
				client.WithSoftwareAndVersion(
					fmt.Sprintf("%s-%s", meta.ENName, meta.Nickname), meta.Version,
				),
				client.WithHandler(recvHandler),
			)
			// Connect client
			if err := Client.Connect(); err != nil {
				continue
			}

			// Subscribe for uplink
			ch, closeFn := Stream.Subscribe()
			go sendHandler(ch)

			// Waiting
			Client.Wait()
			Client = nil
			closeFn()
		}
	}
}

// reload the uplink
func reload() {
	// Stop
	stop = true
	Client.Close()

	// Restart uplink
	go selectUplink()
}
