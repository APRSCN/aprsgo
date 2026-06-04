package uplink

import (
	"sync/atomic"
	"time"

	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/internal/pkg/historydb"
)

// Stats holds the uplink's cumulative counters and per-second rates. It uses
// the shared atomic Counters type so the receive/send handlers, the rate
// updater and the status HTTP handler can touch it concurrently without a data
// race.
var Stats = new(model.Counters)

// lastRX holds the time the most recent packet was received from the uplink.
// It is read by the status handler and written by the receive handler, so it
// is published through an atomic pointer to avoid a data race.
var lastRX atomic.Pointer[time.Time]

// SetLast records the time of the most recent packet received from the uplink.
func SetLast(t time.Time) { lastRX.Store(&t) }

// LastTime returns the time of the most recent packet received from the
// uplink (zero value if none yet).
func LastTime() time.Time {
	if p := lastRX.Load(); p != nil {
		return *p
	}
	return time.Time{}
}

// rate refreshes the per-second rate fields once a second.
func rate() {
	defer mgrWG.Done()
	stop := currentStop() // capture locally; Reload re-arms the package var
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			Stats.UpdateRates()
		}
	}
}

var StatsPacketRX *historydb.MapFloat64History
var StatsPacketTX *historydb.MapFloat64History
var StatsBytesRX *historydb.MapFloat64History
var StatsBytesTX *historydb.MapFloat64History

// stats is the daemon to record stats data
func stats() {
	defer mgrWG.Done()
	stop := currentStop() // capture locally; Reload re-arms the package var
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			// Get time now
			now := time.Now()
			key := float64(now.UnixNano()) / 1e9

			// Uplink byte rates (nil while disconnected -> zeroes).
			var recvRate, sentRate uint64
			if c := GetClient(); c != nil {
				cs := c.GetStats()
				recvRate = cs.CurrentRecvRate
				sentRate = cs.CurrentSentRate
			}

			snap := Stats.Snapshot()

			// Record packet rx rate
			StatsPacketRX.Record(key, float64(snap.RecvPacketRate))
			StatsPacketRX.ClearByKey(30 * 24 * 60 * 60)

			// Record bytes rx rate
			StatsBytesRX.Record(key, float64(recvRate))
			StatsBytesRX.ClearByKey(30 * 24 * 60 * 60)

			// Record packet tx rate
			StatsPacketTX.Record(key, float64(snap.SendPacketRate))
			StatsPacketTX.ClearByKey(30 * 24 * 60 * 60)

			// Record bytes tx rate
			StatsBytesTX.Record(key, float64(sentRate))
			StatsBytesTX.ClearByKey(30 * 24 * 60 * 60)
		}
	}
}
