package uplink

import (
	"strings"
	"time"

	"github.com/APRSCN/aprsgo/internal/pkg/historydb"
	"github.com/APRSCN/aprsutils/client"
	"github.com/APRSCN/aprsutils/parser"
)

var dupRecords *historydb.DupeChecker

// SweepDupes prunes expired entries from the uplink duplicate checker. It is
// intended to be called periodically (e.g. from cron) and is a no-op before
// the uplink daemon has been initialised.
func SweepDupes() {
	if dupRecords != nil {
		dupRecords.Cleanup()
	}
}

// recvHandler is the packet handler of uplink
func recvHandler(packet string) {
	now := time.Now()

	Stats.AddReceivedPackets(1)

	if dupRecords.Seen(packet) {
		Stats.AddReceivedDups(1)
		return
	}

	parsed, _ := parser.Parse(packet, parser.WithDisableToCallsignValidate())
	if parsed.To == "" {
		Stats.AddReceivedErrors(1)
		return
	}

	Stream.Write(parsed, WriterUplink)
	SetLast(now)
}

// sendHandler relays the distribution stream to one uplink client for the
// lifetime of that link.
func sendHandler(c *client.Client, dataCh <-chan StreamData) {
	for data := range dataCh {
		// Never relay uplink- or peer-sourced traffic back upstream
		// (no upstream<->peer cross-feed; no echo to the uplink itself).
		if data.Writer == WriterUplink || isPeerWriter(data.Writer) {
			continue
		}
		if c == nil {
			continue
		}
		if err := c.SendPacket(data.Data.Raw); err != nil {
			continue
		}
		// Count packet tx
		Stats.AddSentPackets(1)
	}
}

// isPeerWriter reports whether a stream writer tag denotes a core-peer source.
func isPeerWriter(writer string) bool {
	return strings.HasPrefix(writer, WriterPeerPrefix)
}
