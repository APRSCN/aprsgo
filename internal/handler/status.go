package handler

import (
	"fmt"
	"runtime"
	"time"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/listener"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/internal/system"
	"github.com/APRSCN/aprsgo/internal/uplink"
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
	if cpuModel == "" {
		cpuModel = config.C.GetString("server.model")
	}

	// Get uplink
	uplinkLast := ""
	var packetRX uint64 = 0
	var packetRXSpeed uint64 = 0
	var packetTX uint64 = 0
	var packetTXSpeed uint64 = 0
	for {
		// Get time of last packet
		uplinkLastByte, err := historydb.C.Get([]byte("uplink.last"))
		if err != nil {
			continue
		}
		uplinkLast = string(uplinkLastByte)

		// Get rx count
		value, err := historydb.GetValue("uplink.packet.rx.count")
		if err != nil {
			continue
		}
		packetRX = uint64(value)

		// Get rx speed
		rxRecent, err := historydb.GetDataSlice("uplink.packet.rx.speed")
		if err != nil {
			continue
		}
		packetRXSpeed = uint64(len(rxRecent))

		// Get tx count
		value, err = historydb.GetValue("uplink.packet.tx.count")
		if err != nil {
			continue
		}
		packetTX = uint64(value)

		// Get rx speed
		txRecent, err := historydb.GetDataSlice("uplink.packet.tx.speed")
		if err != nil {
			continue
		}
		packetTXSpeed = uint64(len(txRecent))

		break
	}

	// Get listeners
	// TODO: Return status here
	listeners := make([]model.ReturnListener, 0)
	for _, l := range listener.Listeners {
		if l.Visible == "hidden" {
			continue
		}
		listeners = append(listeners, model.ReturnListener{
			Name:     l.Name,
			Mode:     l.Type,
			Protocol: l.Protocol,
			Host:     l.Host,
			Port:     l.Port,
		})
	}

	// Get clients
	// TODO: Return status here
	clients := make([]model.ReturnClient, 0)
	for _, v := range listener.Clients {
		clients = append(clients, model.ReturnClient{
			At:       v.At,
			ID:       v.ID,
			Addr:     v.Addr,
			Uptime:   v.Uptime,
			Last:     v.Last,
			Software: v.Software,
			Version:  v.Version,
			Filter:   v.Filter,
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
			Memory:   system.Status.Memory,
		},
		Uplink: model.ReturnUplink{
			ID:            uplink.Client.Callsign(),
			Mode:          uplink.Client.Mode(),
			Protocol:      uplink.Client.Protocol(),
			Host:          uplink.Client.Host(),
			Port:          uplink.Client.Port(),
			Server:        uplink.Client.Server(),
			Up:            uplink.Client.Up(),
			Uptime:        uplink.Client.Uptime(),
			Last:          uplinkLast,
			PacketRX:      packetRX,
			PacketRXSpeed: packetRXSpeed,
			PacketTX:      packetTX,
			PacketTXSpeed: packetTXSpeed,
			BytesRX:       uplink.Client.GetStats().TotalRecvBytes,
			BytesRXSpeed:  uplink.Client.GetStats().CurrentRecvRate,
			BytesTX:       uplink.Client.GetStats().TotalSentBytes,
			BytesTXSpeed:  uplink.Client.GetStats().CurrentSentRate,
		},
		Listeners: listeners,
		Clients:   clients,
	})
}
