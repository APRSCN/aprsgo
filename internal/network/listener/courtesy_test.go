package listener

import (
	"testing"

	"github.com/APRSCN/aprsgo/internal/pkg/historydb"
	"github.com/APRSCN/aprsutils/client"
	"github.com/APRSCN/aprsutils/parser"
)

// parsePkt parses a raw packet for tests.
func parsePkt(t *testing.T, raw string) parser.Parsed {
	t.Helper()
	p, err := parser.Parse(raw, parser.WithDisableToCallsignValidate())
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return p
}

// TestCourtesyPosition verifies that after a message addressed to the client is
// delivered, a single following position from the message's source is passed
// through even though no filter would match it, and only once.
func TestCourtesyPosition(t *testing.T) {
	c := &TCPAPRSClient{
		callSign: "MYCALL",
		mode:     client.IGate,
		heard:    historydb.NewHeardList(),
		courtesy: historydb.NewHeardList(),
		// no server and no compiled filter: passesFilter is always false, so
		// any delivery here is due to message routing / courtesy logic.
	}
	snap := deliverState{
		loggedIn:  true,
		connected: true,
		callSign:  "MYCALL",
		mode:      client.IGate,
	}

	// A message addressed to the client from REMOTE is delivered (message
	// routing) and records REMOTE as a courtesy candidate.
	msg := parsePkt(t, "REMOTE>APRS,TCPIP*,qAC,SERVER::MYCALL   :hello{1")
	if !c.shouldDeliver(snap, msg) {
		t.Fatal("message addressed to client should be delivered")
	}

	// REMOTE's next position is passed through even without a matching filter.
	pos := parsePkt(t, "REMOTE>APRS,TCPIP*,qAC,SERVER:!4903.50N/07201.75W-")
	if !c.shouldDeliver(snap, pos) {
		t.Fatal("courtesy position from message source should be delivered")
	}

	// The courtesy is one-shot: a second position is no longer forced through.
	if c.shouldDeliver(snap, pos) {
		t.Fatal("courtesy position should be delivered only once")
	}
}

// TestNoCourtesyWithoutMessage verifies a position from a station that has not
// messaged the client is not force-delivered (no filter, not a courtesy).
func TestNoCourtesyWithoutMessage(t *testing.T) {
	c := &TCPAPRSClient{
		callSign: "MYCALL",
		mode:     client.IGate,
		heard:    historydb.NewHeardList(),
		courtesy: historydb.NewHeardList(),
	}
	snap := deliverState{loggedIn: true, connected: true, callSign: "MYCALL", mode: client.IGate}

	pos := parsePkt(t, "STRANGER>APRS,TCPIP*,qAC,SERVER:!4903.50N/07201.75W-")
	if c.shouldDeliver(snap, pos) {
		t.Fatal("position from a non-correspondent must not be force-delivered")
	}
}
