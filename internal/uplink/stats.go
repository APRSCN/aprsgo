package uplink

import (
	"time"

	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/model"
)

var Stats = new(model.Statistics)
var Last time.Time

// rate is the daemon to refresh rate
func rate() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Update rates
			currentSentPackets := Stats.SentPackets
			currentReceivedPackets := Stats.ReceivedPackets

			Stats.SendPacketRate = currentSentPackets - Stats.LastSentPackets
			Stats.RecvPacketRate = currentReceivedPackets - Stats.LastReceivedPackets

			Stats.LastSentPackets = currentSentPackets
			Stats.LastReceivedPackets = currentReceivedPackets
		}
	}
}

var StatsPacketRX *historydb.MapFloat64History
var StatsPacketTX *historydb.MapFloat64History
var StatsBytesRX *historydb.MapFloat64History
var StatsBytesTX *historydb.MapFloat64History

// stats is the daemon to record stats data
func stats() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Get time now
			now := time.Now()

			// Record packet rx rate
			StatsPacketRX.Record(float64(now.UnixNano())/1e9, float64(Stats.RecvPacketRate))
			StatsPacketRX.ClearByKey(30 * 24 * 60 * 60)

			// Record bytes rx rate
			StatsBytesRX.Record(float64(now.UnixNano())/1e9, float64(Client.GetStats().CurrentRecvRate))
			StatsBytesRX.ClearByKey(30 * 24 * 60 * 60)

			// Record packet tx rate
			StatsPacketTX.Record(float64(now.UnixNano())/1e9, float64(Stats.SendPacketRate))
			StatsPacketTX.ClearByKey(30 * 24 * 60 * 60)

			// Record bytes tx rate
			StatsBytesTX.Record(float64(now.UnixNano())/1e9, float64(Client.GetStats().CurrentSentRate))
			StatsPacketTX.ClearByKey(30 * 24 * 60 * 60)
		}
	}
}
