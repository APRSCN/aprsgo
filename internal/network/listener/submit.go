package listener

import (
	"errors"
	"strings"
	"time"

	"github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/network/uplink"
	"github.com/APRSCN/aprsgo/internal/pkg/historydb"
	"github.com/APRSCN/aprsgo/internal/security"
	"github.com/APRSCN/aprsutils"
	"github.com/APRSCN/aprsutils/parser"
	"github.com/APRSCN/aprsutils/qConstruct"
)

// Submit-related errors returned by ProcessSubmit so callers (UDP/HTTP) can
// distinguish failure modes and produce appropriate responses/statistics.
var (
	ErrSubmitParse     = errors.New("packet parse failure")
	ErrSubmitDuplicate = errors.New("duplicate packet")
	ErrSubmitQDrop     = errors.New("dropped by q-construct (loop/invalid)")
	ErrSubmitTooShort  = errors.New("packet too short")
)

// submitDedup is the shared duplicate-suppression window for connectionless
// submit endpoints (UDP/HTTP). TCP clients keep their own per-connection store.
var submitDedup = historydb.NewDupeChecker(30 * time.Second)

// SweepSubmitDedup prunes expired entries from the shared submit duplicate
// checker. It is intended to be called periodically (e.g. from cron).
func SweepSubmitDedup() { submitDedup.Cleanup() }

// SubmitSource identifies how a submitted packet arrived, which selects the
// q-construct connection type (and therefore the qAU/qAC injected by the
// server).
type SubmitSource int

const (
	// SubmitUDP is a UDP datagram submit -> qAU.
	SubmitUDP SubmitSource = iota
	// SubmitHTTP is an HTTP POST submit -> qAC (treated as a verified TCP client).
	SubmitHTTP
)

// ProcessSubmit validates, de-duplicates, q-processes and injects a single
// submitted packet originating from a connectionless endpoint (UDP/HTTP).
//
// callsign is the authenticated login from the submit envelope; verified
// indicates the passcode checked out. The function returns nil on success or
// one of the ErrSubmit* errors describing why the packet was not injected.
func ProcessSubmit(callsign string, verified bool, packet string, src SubmitSource) error {
	packet = strings.TrimRight(packet, "\r\n")
	if len(packet) < 2 || !strings.Contains(packet, ">") {
		return ErrSubmitTooShort
	}

	// Duplicate suppression (shared 30s window).
	if submitDedup.Seen(packet) {
		return ErrSubmitDuplicate
	}

	// Initial parse (lenient on toCall, like the TCP path).
	parsed, _ := parser.Parse(packet, parser.WithDisableToCallsignValidate())
	if parsed.To == "" {
		return ErrSubmitParse
	}

	// Reject packets from blacklisted / bogus source callsigns.
	if !security.SourceAllowed(parsed.From) {
		return ErrSubmitParse
	}

	// Select connection type per submit source.
	connType := qConstruct.ConnectionVerified
	if src == SubmitUDP {
		connType = qConstruct.ConnectionDirectUDP
	}

	qConfig := &qConstruct.QConfig{
		ServerLogin:            config.Get().Server.ID,
		ClientLogin:            callsign,
		ConnectionType:         connType,
		IsVerified:             verified,
		QProtocolID:            security.QProtocolID(),
		DisallowOtherProtocols: security.DisallowOtherQProtocols(),
	}
	result, err := qConstruct.QConstruct(parsed, qConfig)
	if err != nil || result.ShouldDrop || result.IsLoop {
		return ErrSubmitQDrop
	}

	// Rewrite the path with the q-processed result.
	packet, err = qConstruct.Replace(packet, parsed.To, result.Path)
	if err != nil {
		return ErrSubmitParse
	}

	// Reparse the rewritten packet and inject into the distribution stream.
	parsed, err = parser.Parse(packet, parser.WithDisableToCallsignValidate())
	if err != nil {
		return ErrSubmitParse
	}
	uplink.Stream.Write(parsed, callsign)
	return nil
}

// parseSubmitEnvelope splits a UDP/HTTP submit payload into the login line and
// the packet body. The envelope format is:
//
//	user CALL pass CODE vers SW VER\r\n
//	PACKET\r\n
//
// It returns the parsed login (callsign, verified) and the remaining packet
// lines. ok is false if no login line is present.
func parseSubmitEnvelope(payload string) (callsign string, verified bool, packets []string, ok bool) {
	// Split off the first line (login).
	idx := strings.IndexByte(payload, '\n')
	if idx < 0 {
		return "", false, nil, false
	}
	loginLine := strings.TrimRight(payload[:idx], "\r")
	rest := payload[idx+1:]

	call, pass, ok := parseLoginLine(loginLine)
	if !ok {
		return "", false, nil, false
	}

	// Remaining non-empty lines are packets.
	for _, line := range strings.Split(rest, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) != "" {
			packets = append(packets, line)
		}
	}

	verified = aprsutils.Passcode(call) == pass
	return call, verified, packets, true
}

// parseLoginLine parses a "user CALL pass CODE vers SW VER [filter ...]" line.
// It returns the callsign and integer passcode (0 when absent/invalid).
func parseLoginLine(line string) (callsign string, passcode int, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "user" {
		return "", 0, false
	}
	callsign = fields[1]
	if !aprsutils.ValidateCallsign(callsign) {
		return "", 0, false
	}
	for i := 2; i+1 < len(fields); i++ {
		if fields[i] == "pass" {
			passcode = atoiSafe(fields[i+1])
			break
		}
	}
	return callsign, passcode, true
}

// atoiSafe parses an int, returning -1 for invalid input (so it never matches
// a real passcode).
func atoiSafe(s string) int {
	n := 0
	if s == "" {
		return -1
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// SubmitEnvelopeResult describes the outcome of submitting a full envelope.
type SubmitEnvelopeResult struct {
	Accepted int // packets successfully injected
	Rejected int // packets rejected (parse/loop/dup/short)
}

// SubmitEnvelope parses a UDP/HTTP submit payload (login line + packets) and
// injects each packet. It returns an error only for envelope-level problems
// (missing login, bad callsign, failed passcode); per-packet rejections are
// counted in the result, not returned as an error. src selects the q-construct
// behaviour (qAU for UDP, qAC for HTTP).
func SubmitEnvelope(payload string, src SubmitSource) (SubmitEnvelopeResult, error) {
	var res SubmitEnvelopeResult

	call, verified, packets, ok := parseSubmitEnvelope(payload)
	if !ok {
		return res, errors.New("missing or invalid login envelope")
	}
	if !verified {
		return res, errors.New("invalid passcode")
	}
	if len(packets) == 0 {
		return res, errors.New("no packet data found")
	}

	for _, pkt := range packets {
		if err := ProcessSubmit(call, true, pkt, src); err != nil {
			res.Rejected++
			continue
		}
		res.Accepted++
	}
	return res, nil
}
