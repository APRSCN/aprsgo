package handler

import (
	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/gofiber/fiber/v3"
)

// History returns history as JSON
func History(c fiber.Ctx) error {
	// Get history from DB
	history := model.ReturnHistory{}
	for {
		var err error
		// Get history
		history.System, err = historydb.GetDataSlice("status.system")
		if err != nil {
			continue
		}
		break
	}

	return model.Resp(c, model.ReturnHistory{
		Msg:    "success",
		System: history.System,
	})
}
