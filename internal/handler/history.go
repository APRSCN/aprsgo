package handler

import (
	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/gofiber/fiber/v3"
)

// History returns history as JSON
func History(c fiber.Ctx) error {
	// Get history from DB
	history := model.HistoryReturn{}
	for {
		var err error

		// Get memory
		history.Memory, err = historydb.GetDataSlice("stats.memory")
		if err != nil {
			continue
		}

		// Get uplink packet rx
		history.UplinkPacketRX, err = historydb.GetDataSlice("stats.uplink.packet.rx")
		if err != nil {
			continue
		}

		// TODO: Get uplink packet tx

		// Get uplink bytes rx
		history.UplinkBytesRX, err = historydb.GetDataSlice("stats.uplink.bytes.rx")
		if err != nil {
			continue
		}

		// Get uplink bytes tx
		history.UplinkBytesTX, err = historydb.GetDataSlice("stats.uplink.bytes.tx")
		if err != nil {
			continue
		}

		break
	}

	return model.Resp(c, history)
}
