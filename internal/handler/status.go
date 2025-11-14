package handler

import (
	"fmt"
	"runtime"
	"time"

	"github.com/APRSCN/aprsgo/internal/config"
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
	var up *model.ReturnUplink = nil
	if uplink.Client != nil {
		up = &model.ReturnUplink{
			ID:           uplink.Client.Callsign(),
			Mode:         uplink.Client.Mode(),
			Protocol:     uplink.Client.Protocol(),
			Host:         uplink.Client.Host(),
			Port:         uplink.Client.Port(),
			Server:       uplink.Client.Server(),
			Up:           uplink.Client.Up(),
			Uptime:       uplink.Client.Uptime(),
			Last:         uplink.Last,
			PacketRX:     uplink.Stats.ReceivedPackets,
			PacketRXDup:  uplink.Stats.ReceivedDups,
			PacketRXErr:  uplink.Stats.ReceivedErrors,
			PacketRXRate: uplink.Stats.RecvPacketRate,
			PacketTX:     uplink.Stats.SentPackets,
			PacketTXRate: uplink.Stats.SendPacketRate,
			BytesRX:      uplink.Client.GetStats().TotalRecvBytes,
			BytesRXRate:  uplink.Client.GetStats().CurrentRecvRate,
			BytesTX:      uplink.Client.GetStats().TotalSentBytes,
			BytesTXRate:  uplink.Client.GetStats().CurrentSentRate,
		}
	}

	// Get listeners
	listeners := make([]model.ReturnListener, 0)
	for _, l := range listener.Listeners {
		if l.Visible == "hidden" {
			continue
		}
		listeners = append(listeners, model.ReturnListener{
			Name:         l.Name,
			Mode:         l.Type,
			Protocol:     l.Protocol,
			Host:         l.Host,
			Port:         l.Port,
			Filter:       l.Filter,
			OnlineClient: l.OnlineClient,
			PeakClient:   l.PeakClient,
			PacketRX:     l.Stats.ReceivedPackets,
			PacketRXRate: l.Stats.RecvPacketRate,
			PacketTX:     l.Stats.SentPackets,
			PacketTXRate: l.Stats.SendPacketRate,
			BytesRX:      l.Stats.ReceivedBytes,
			BytesRXRate:  l.Stats.RecvByteRate,
			BytesTX:      l.Stats.SentBytes,
			BytesTXRate:  l.Stats.SendByteRate,
		})
	}

	// Get clients
	clients := make([]model.ReturnClient, 0)
	for _, v := range listener.Clients {
		clients = append(clients, model.ReturnClient{
			At:           v.At,
			ID:           v.ID,
			Verified:     v.Verified,
			Addr:         v.Addr,
			Uptime:       v.Uptime,
			Last:         v.Last,
			Software:     v.Software,
			Version:      v.Version,
			Filter:       v.Filter,
			PacketRX:     v.Stats.ReceivedPackets,
			PacketRXDup:  v.Stats.ReceivedDups,
			PacketRXErr:  v.Stats.ReceivedErrors,
			PacketRXRate: v.Stats.RecvPacketRate,
			PacketTX:     v.Stats.SentPackets,
			PacketTXRate: v.Stats.SendPacketRate,
			BytesRX:      v.Stats.ReceivedBytes,
			BytesRXRate:  v.Stats.RecvByteRate,
			BytesTX:      v.Stats.SentBytes,
			BytesTXRate:  v.Stats.SendByteRate,
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
		Uplink:    up,
		Listeners: listeners,
		Clients:   clients,
	})
}
