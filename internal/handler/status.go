package handler

import (
	"fmt"
	"runtime"
	"time"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/listener"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/internal/system"
	"github.com/APRSCN/aprsgo/pkg/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/shirou/gopsutil/v4/cpu"
)

// Status returns status as JSON
func Status(c fiber.Ctx) error {
	// Get time now
	timeNow := time.Now()

	// Get system status
	var cpuModel string
	var cpuNum int32 = 0
	for {
		info, err := cpu.Info()
		if err != nil {
			continue
		}
		cpuModel = info[0].ModelName

		for _, u := range info {
			cpuNum += u.Cores
		}

		if cpuNum > 1 {
			cpuModel = fmt.Sprintf("%d x %s", cpuNum, cpuModel)
		}
		break
	}

	// Get listeners
	// TODO: ReturnStatus status here
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

	return model.Resp(c, model.ReturnStatus{
		Msg: "success",
		Server: model.ReturnServer{
			Admin:    config.C.GetString("admin.name"),
			Email:    config.C.GetString("admin.email"),
			OS:       utils.PrettierOSName(),
			Arch:     runtime.GOARCH,
			ID:       config.C.GetString("server.id"),
			Software: config.ENName,
			Version:  fmt.Sprintf("%s %s", config.Version, config.Nickname),
			Now:      timeNow,
			Uptime:   timeNow.Sub(config.Uptime).Seconds(),
			Model:    cpuModel,
			Percent:  system.Status.Percent,
			Total:    system.Status.Total,
			Used:     system.Status.Used,
		},
		Listeners: listeners,
	})
}
