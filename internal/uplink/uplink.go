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

// InitUplink inits uplink daemon
func InitUplink() {
	go selectUplink()
}

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

	// Select available node
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
			client.WithHandler(func(packet string) {
				//fmt.Println(packet)
			}),
		)
		// Connect client
		err = Client.Connect()
		if err != nil {
			continue
		}
	}
	defer Client.Close()
	select {}
}
