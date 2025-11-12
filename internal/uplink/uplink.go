package uplink

import (
	"fmt"
	"strconv"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsutils/client"
	"github.com/ghinknet/json"
	"go.uber.org/zap"
)

var Client *client.Client
var Stream *DataStream

// InitUplink inits uplink daemon
func InitUplink() {
	// Init Stream
	Stream = NewDataStream(100)

	// Start uplink
	go selectUplink()

	// Start stats
	go stats()

	logger.L.Debug("Uplink daemon initialized")
}

// selectUplink is the daemon of uplink that will automatically choose an available uplink
func selectUplink() {
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
	for {
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
				client.WithHandler(packetHandler),
			)
			// Connect client
			err = Client.Connect()
			if err != nil {
				continue
			}

			// Waiting
			Client.Wait()
		}
	}
}
