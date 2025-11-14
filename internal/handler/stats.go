package handler

import (
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/internal/system"
	"github.com/APRSCN/aprsgo/internal/uplink"
	"github.com/gofiber/fiber/v3"
)

// Stats returns stats as JSON
func Stats(c fiber.Ctx) error {
	// Get stats from DB
	stats := model.StatsReturn{}
	for {
		// Get memory
		stats.Memory = system.StatsMemory.ToSlice()

		// Get uplink packet rx
		stats.UplinkPacketRX = uplink.StatsPacketRX.ToSlice()

		// Get uplink packet tx
		stats.UplinkPacketTX = uplink.StatsPacketTX.ToSlice()

		// Get uplink bytes rx
		stats.UplinkBytesRX = uplink.StatsBytesRX.ToSlice()

		// Get uplink bytes tx
		stats.UplinkBytesTX = uplink.StatsBytesTX.ToSlice()

		break
	}

	return model.Resp(c, stats)
}
