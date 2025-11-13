package handler

import (
	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/gofiber/fiber/v3"
)

// Stats returns stats as JSON
func Stats(c fiber.Ctx) error {
	// Get stats from DB
	stats := model.StatsReturn{}
	for {
		var err error

		// Get memory
		stats.Memory, err = historydb.GetDataSlice("stats.memory")
		if err != nil {
			continue
		}

		// Get uplink packet rx
		stats.UplinkPacketRX, err = historydb.GetDataSlice("stats.uplink.packet.rx")
		if err != nil {
			continue
		}

		// Get uplink packet tx
		stats.UplinkPacketTX, err = historydb.GetDataSlice("stats.uplink.packet.tx")
		if err != nil {
			continue
		}

		// Get uplink bytes rx
		stats.UplinkBytesRX, err = historydb.GetDataSlice("stats.uplink.bytes.rx")
		if err != nil {
			continue
		}

		// Get uplink bytes tx
		stats.UplinkBytesTX, err = historydb.GetDataSlice("stats.uplink.bytes.tx")
		if err != nil {
			continue
		}

		break
	}

	return model.Resp(c, stats)
}
