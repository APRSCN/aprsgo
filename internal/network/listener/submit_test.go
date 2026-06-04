package listener

import (
	"testing"
	"time"

	config2 "github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/network/uplink"
)

func TestParseLoginLine(t *testing.T) {
	cases := []struct {
		line     string
		wantCall string
		wantPass int
		wantOK   bool
	}{
		{"user TEST pass 29939 vers sw 1.0", "TEST", 29939, true},
		{"user N0CALL pass 12345", "N0CALL", 12345, true},
		{"user TEST vers sw 1.0", "TEST", 0, true}, // no pass -> 0
		{"hello world", "", 0, false},
		{"user", "", 0, false},
		{"user TE/ST pass 1", "", 0, false}, // invalid callsign
	}
	for _, c := range cases {
		call, pass, ok := parseLoginLine(c.line)
		if ok != c.wantOK || call != c.wantCall || pass != c.wantPass {
			t.Errorf("parseLoginLine(%q) = (%q,%d,%v), want (%q,%d,%v)",
				c.line, call, pass, ok, c.wantCall, c.wantPass, c.wantOK)
		}
	}
}

func TestParseSubmitEnvelope(t *testing.T) {
	// TEST has passcode 29939.
	payload := "user TEST pass 29939 vers sw 1.0\r\nTEST>APRS,TCPIP*:>hello\r\n"
	call, verified, packets, ok := parseSubmitEnvelope(payload)
	if !ok {
		t.Fatal("envelope should parse")
	}
	if call != "TEST" {
		t.Errorf("call = %q, want TEST", call)
	}
	if !verified {
		t.Error("verified should be true for correct passcode")
	}
	if len(packets) != 1 || packets[0] != "TEST>APRS,TCPIP*:>hello" {
		t.Errorf("packets = %v, want [TEST>APRS,TCPIP*:>hello]", packets)
	}

	// Wrong passcode -> not verified.
	_, verified, _, ok = parseSubmitEnvelope("user TEST pass 1 vers sw 1.0\r\nX>Y:>z\r\n")
	if !ok || verified {
		t.Errorf("wrong passcode should parse but be unverified (ok=%v verified=%v)", ok, verified)
	}

	// No newline -> not ok.
	if _, _, _, ok := parseSubmitEnvelope("user TEST pass 29939"); ok {
		t.Error("payload without newline must not parse")
	}
}

// TestSubmitEnvelopeInjectsUDP verifies a UDP submit produces a qAU construct
// and the packet reaches the distribution stream.
func TestSubmitEnvelopeInjectsUDP(t *testing.T) {
	config2.Set(testConfig())
	uplink.Stream = uplink.NewDataStream(10)
	ch, unsub := uplink.Stream.Subscribe()
	defer unsub()

	payload := "user TEST pass 29939 vers sw 1.0\r\nTEST>APRS,TCPIP*:>udp content\r\n"
	res, err := SubmitEnvelope(payload, SubmitUDP)
	if err != nil {
		t.Fatalf("SubmitEnvelope: %v", err)
	}
	if res.Accepted != 1 {
		t.Fatalf("accepted = %d, want 1 (rejected=%d)", res.Accepted, res.Rejected)
	}

	select {
	case data := <-ch:
		// UDP submit must carry a qAU construct.
		foundQAU := false
		for _, hop := range data.Data.Path {
			if hop == "qAU" {
				foundQAU = true
			}
		}
		if !foundQAU {
			t.Errorf("expected qAU in path, got %v", data.Data.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("packet not delivered to stream")
	}
}

// TestSubmitEnvelopeInjectsHTTP verifies an HTTP submit produces a qAC construct.
func TestSubmitEnvelopeInjectsHTTP(t *testing.T) {
	config2.Set(testConfig())
	uplink.Stream = uplink.NewDataStream(10)
	ch, unsub := uplink.Stream.Subscribe()
	defer unsub()

	payload := "user TEST pass 29939 vers sw 1.0\r\nTEST>APRS,TCPIP*:>http content\r\n"
	res, err := SubmitEnvelope(payload, SubmitHTTP)
	if err != nil {
		t.Fatalf("SubmitEnvelope: %v", err)
	}
	if res.Accepted != 1 {
		t.Fatalf("accepted = %d, want 1", res.Accepted)
	}

	select {
	case data := <-ch:
		foundQAC := false
		for _, hop := range data.Data.Path {
			if hop == "qAC" {
				foundQAC = true
			}
		}
		if !foundQAC {
			t.Errorf("expected qAC in path, got %v", data.Data.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("packet not delivered to stream")
	}
}

func TestSubmitEnvelopeRejectsBadPasscode(t *testing.T) {
	config2.Set(testConfig())
	uplink.Stream = uplink.NewDataStream(10)

	_, err := SubmitEnvelope("user TEST pass 1 vers sw 1.0\r\nTEST>APRS:>x\r\n", SubmitUDP)
	if err == nil {
		t.Error("expected error for invalid passcode")
	}
}

// testConfig returns a minimal config with a server ID for submit tests.
func testConfig() config2.StaticConfig {
	var c config2.StaticConfig
	c.Server.ID = "TESTING"
	c.Server.BuffSize = 128
	return c
}
