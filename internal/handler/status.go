package handler

import (
	"fmt"
	"runtime"
	"time"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/listener"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/pkg/utils"
	"github.com/gofiber/fiber/v3"
)

// Status returns status as JSON
func Status(c fiber.Ctx) error {
	// Get time now
	timeNow := time.Now()

	// Get listeners
	// TODO: Return status here
	listeners := make([]model.ReturnListener, 0)
	for _, l := range listener.Listeners {
		if l.Visible == "hidden" {
			continue
		}
		listeners = append(listeners, model.ReturnListener{
			Name:     l.Name,
			Type:     l.Type,
			Protocol: l.Protocol,
			Host:     l.Host,
			Port:     l.Port,
		})
	}

	return model.Resp(c, model.Return{
		Msg: "success",
		Server: model.ReturnServer{
			Admin:    config.C.GetString("admin.name"),
			Email:    config.C.GetString("admin.email"),
			OS:       fmt.Sprintf("%s %s", utils.PrettierOSName(), runtime.GOARCH),
			ID:       config.C.GetString("server.id"),
			Software: config.ENName,
			Version:  fmt.Sprintf("%s %s", config.Version, config.Nickname),
			TimeNow:  timeNow.Unix(),
			Uptime:   int64(timeNow.Sub(config.Uptime).Seconds()),
		},
		Listeners: listeners,
	})
}
