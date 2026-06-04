package handler

import (
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/internal/network/uplink"
	"github.com/APRSCN/aprsgo/internal/system"
	"github.com/gofiber/fiber/v3"
)

// Stats returns stats as JSON
func Stats(c fiber.Ctx) error {
	stats := model.StatsReturn{
		Memory:         system.StatsMemory.ToSlice(),
		UplinkPacketRX: uplink.StatsPacketRX.ToSlice(),
		UplinkPacketTX: uplink.StatsPacketTX.ToSlice(),
		UplinkBytesRX:  uplink.StatsBytesRX.ToSlice(),
		UplinkBytesTX:  uplink.StatsBytesTX.ToSlice(),
	}

	return model.RespSuccess(c, stats)
}
