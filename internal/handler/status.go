package handler

import (
	"fmt"
	"runtime"
	"time"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/pkg/utils"
	"github.com/gofiber/fiber/v3"
)

// Status returns status as JSON
func Status(c fiber.Ctx) error {
	// Get time now
	timeNow := time.Now()

	return model.Resp(c, model.Return{
		Msg: "success",
		Server: model.Server{
			Admin:    config.C.GetString("admin.name"),
			Email:    config.C.GetString("admin.email"),
			OS:       fmt.Sprintf("%s %s", utils.PrettierOSName(), runtime.GOARCH),
			ID:       config.C.GetString("server.id"),
			Software: config.ENName,
			Version:  fmt.Sprintf("%s %s", config.Version, config.Nickname),
			TimeNow:  timeNow.Unix(),
			Uptime:   int64(timeNow.Sub(config.Uptime).Seconds()),
		},
	})
}
