package uplink

import (
	"fmt"
	"strconv"
	"time"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsutils/client"
	"github.com/ghinknet/json"
	"go.uber.org/zap"
)

var Client *client.Client
var Stream *DataStream
var Last time.Time
var Stats = new(model.Statistics)
var stop = false
var dupRecords *historydb.DupRecord

// InitUplink inits uplink daemon
func InitUplink() {
	// Init Stream
	Stream = NewDataStream(100)

	// Init dupRecords
	dupRecords = historydb.NewDup()

	// Add config change trigger
	config.OnChange = append(config.OnChange, func() {
		reload()
	})

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

	// Load config
	var uplinksConfig []model.UplinkConfig
	marshalled, err := json.Marshal(config.C.Get("server.uplinks"))
	if err != nil {
		logger.L.Error("Error loading uplinks config", zap.Error(err))
		return
	}
	err = json.Unmarshal(marshalled, &uplinksConfig)
	if err != nil {
		logger.L.Error("Error loading uplinks config", zap.Error(err))
	}

	defer Client.Close()
	// Select available node with fallthrough
	for !stop {
		for _, uplink := range uplinksConfig {
			// Create client
			Client = client.NewClient(
				config.C.GetString("server.id"),
				strconv.Itoa(config.C.GetInt("server.passcode")),
				uplink.Mode, uplink.Protocol, uplink.Host, uplink.Port,
				client.WithLogger(&ZapLogger{logger: logger.L}),
				client.WithSoftwareAndVersion(
					fmt.Sprintf("%s-%s", config.ENName, config.Nickname), config.Version,
				),
				client.WithHandler(recvHandler),
			)
			// Connect client
			err = Client.Connect()
			if err != nil {
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
