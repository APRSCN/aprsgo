package handler

import (
	"fmt"
	"runtime"
	"time"

	"github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/meta"
	"github.com/APRSCN/aprsgo/internal/model"
	listener2 "github.com/APRSCN/aprsgo/internal/network/listener"
	"github.com/APRSCN/aprsgo/internal/network/peer"
	uplink2 "github.com/APRSCN/aprsgo/internal/network/uplink"
	"github.com/APRSCN/aprsgo/internal/pkg/historydb"
	"github.com/APRSCN/aprsgo/internal/pkg/utils"
	"github.com/APRSCN/aprsgo/internal/system"
	"github.com/gofiber/fiber/v3"
	"github.com/shirou/gopsutil/v4/cpu"
)

// Status returns status as JSON
func Status(c fiber.Ctx) error {
	// Get time now
	timeNow := time.Now()

	// Get system status. cpu.Info() can transiently fail; retry a few times
	// with a short backoff instead of spinning, and tolerate an empty result.
	var cpuModel string
	var cpuNum int32 = 0
	for attempt := 0; attempt < 3; attempt++ {
		info, err := cpu.Info()
		if err != nil || len(info) == 0 {
			time.Sleep(50 * time.Millisecond)
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
		cpuModel = config.Get().Server.Model
	}

	// Get uplink
	var up *model.ReturnUplink = nil
	if uc := uplink2.GetClient(); uc != nil {
		us := uplink2.Stats.Snapshot()
		cs := uc.GetStats()
		up = &model.ReturnUplink{
			ID:           uc.Callsign(),
			Mode:         uc.Mode(),
			Protocol:     uc.Protocol(),
			Host:         uc.Host(),
			RealAddr:     uc.RemoteAddr(),
			Port:         uc.Port(),
			ServerID:     uc.ServerID(),
			Server:       uc.Server(),
			Up:           uc.Up(),
			Uptime:       uc.Uptime(),
			Last:         uplink2.LastTime(),
			PacketRX:     us.ReceivedPackets,
			PacketRXDup:  us.ReceivedDups,
			PacketRXErr:  us.ReceivedErrors,
			PacketRXRate: us.RecvPacketRate,
			PacketTX:     us.SentPackets,
			PacketTXRate: us.SendPacketRate,
			BytesRX:      cs.TotalRecvBytes,
			BytesRXRate:  cs.CurrentRecvRate,
			BytesTX:      cs.TotalSentBytes,
			BytesTXRate:  cs.CurrentSentRate,
		}
	}

	// Get listeners
	listeners := make([]*model.ReturnListener, 0)
	for _, l := range listener2.ListenersSnapshot() {
		if l.Visible == "hidden" {
			continue
		}
		st := l.Stats() // atomic snapshot
		listeners = append(listeners, &model.ReturnListener{
			Name:         l.Name,
			Mode:         l.Type,
			Protocol:     l.Protocol,
			Host:         l.Host,
			Port:         l.Port,
			Filter:       l.Filter,
			OnlineClient: l.OnlineClient(),
			PeakClient:   l.PeakClient(),
			PacketRX:     st.ReceivedPackets,
			PacketRXRate: st.RecvPacketRate,
			PacketTX:     st.SentPackets,
			PacketTXRate: st.SendPacketRate,
			BytesRX:      st.ReceivedBytes,
			BytesRXRate:  st.RecvByteRate,
			BytesTX:      st.SentBytes,
			BytesTXRate:  st.SendByteRate,
		})
	}

	// Get clients
	clients := make([]*model.ReturnClient, 0)
	for _, v := range listener2.ClientsSnapshot() {
		clients = append(clients, &model.ReturnClient{
			At:           v.At,
			Port:         v.Port,
			ID:           v.ID,
			Verified:     v.Verified,
			Addr:         v.Addr,
			Uptime:       v.Uptime,
			Last:         v.Last,
			Software:     v.Software,
			Version:      v.Version,
			Filter:       v.Filter,
			OutQ:         v.OutQ,
			MsgRcpts:     v.MsgRcpts,
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

	// Get core peers
	peers := make([]*model.ReturnPeer, 0)
	for _, p := range peer.List() {
		peers = append(peers, &model.ReturnPeer{
			Name: p.Name,
			ID:   p.ID,
			Addr: p.Addr,
		})
	}

	sysStatus := system.Snapshot()

	gs := listener2.GlobalStats()
	totals := model.ReturnTotals{
		Clients:       listener2.GlobalClientCount(),
		PacketRX:      gs.ReceivedPackets,
		PacketTX:      gs.SentPackets,
		BytesRX:       gs.ReceivedBytes,
		BytesTX:       gs.SentBytes,
		PacketRXRate:  gs.RecvPacketRate,
		PacketTXRate:  gs.SendPacketRate,
		BytesRXRate:   gs.RecvByteRate,
		BytesTXRate:   gs.SendByteRate,
		Dupes:         gs.ReceivedDups,
		PositionCache: historydb.Positions.Len(),
	}

	return model.RespSuccess(c, model.ReturnStatus{
		Msg: "success",
		Server: model.ReturnServer{
			Admin:    config.Get().Admin.Name,
			Email:    config.Get().Admin.Email,
			OS:       utils.PrettierOSName(),
			Arch:     runtime.GOARCH,
			ID:       config.Get().Server.ID,
			Software: meta.ENName,
			Version:  fmt.Sprintf("%s %s", meta.Version, meta.Nickname),
			Now:      timeNow,
			Uptime:   timeNow.Sub(meta.StartAt).Seconds(),
			Model:    cpuModel,
			Percent:  sysStatus.Percent,
			Memory:   sysStatus.Memory,
		},
		Totals:    totals,
		Uplink:    up,
		Peers:     peers,
		Listeners: listeners,
		Clients:   clients,
	})
}
