package peer

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	config2 "github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"github.com/APRSCN/aprsgo/internal/network/uplink"
	"github.com/APRSCN/aprsutils/parser"
	"go.uber.org/zap"
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

func testConfig() config2.StaticConfig {
	var c config2.StaticConfig
	c.Server.ID = "TESTING"
	c.Server.BuffSize = 128
	return c
}

// newManager builds a Manager bound to a free local port with one fake UDP
// peer.
func newManager(t *testing.T, peerAddr *net.UDPAddr) *Manager {
	t.Helper()
	m := &Manager{bindHost: "127.0.0.1", bindPort: 0, stop: make(chan struct{})}
	m.peers = []*remotePeer{{name: "fake", id: "SRV1", udpAddr: peerAddr}}
	if err := m.start(); err != nil {
		t.Fatalf("start peer manager: %v", err)
	}
	return m
}

// TestPeerInboundInjectsToStream verifies a packet sent by a peer is injected
// into the distribution stream (tagged as a peer source) and q-processed.
func TestPeerInboundInjectsToStream(t *testing.T) {
	logger.L = zap.NewNop()
	config2.Set(testConfig())
	uplink.Stream = uplink.NewDataStream(10)
	ch, unsub := uplink.Stream.Subscribe()
	defer unsub()

	// Fake remote peer socket (its address is what the manager recognises).
	fake, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("fake peer listen: %v", err)
	}
	defer fake.Close()
	fakeAddr := fake.LocalAddr().(*net.UDPAddr)

	m := newManager(t, fakeAddr)
	defer m.shutdown()

	// Send a packet from the fake peer to the manager's socket.
	mgrAddr := m.udpConn.LocalAddr().(*net.UDPAddr)
	pkt := "SRC>DST,qAR,IGATE:peer to client"
	if _, err := fake.WriteToUDP([]byte(pkt+"\r\n"), mgrAddr); err != nil {
		t.Fatalf("fake peer write: %v", err)
	}

	select {
	case data := <-ch:
		if !strings.HasPrefix(data.Writer, WriterPrefix) {
			t.Errorf("writer = %q, want peer:* prefix", data.Writer)
		}
		// qAR packet passes through unchanged (no server append for qAR).
		if data.Data.From != "SRC" {
			t.Errorf("From = %q, want SRC", data.Data.From)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("peer packet not injected into stream")
	}
}

// TestPeerOutboundRelay verifies a client-sourced packet is relayed out to the
// configured peer, and that uplink/peer-sourced packets are NOT.
func TestPeerOutboundRelay(t *testing.T) {
	logger.L = zap.NewNop()
	config2.Set(testConfig())
	uplink.Stream = uplink.NewDataStream(10)

	// Fake remote peer to receive relayed packets.
	fake, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("fake peer listen: %v", err)
	}
	defer fake.Close()
	fakeAddr := fake.LocalAddr().(*net.UDPAddr)

	m := newManager(t, fakeAddr)
	defer m.shutdown()

	// A client-sourced packet should be relayed to the peer.
	uplink.Stream.Write(parsePkt(t, "SRC>DST,qAR,IGATE:from client"), "N5CAL-1")

	buf := make([]byte, 1024)
	_ = fake.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := fake.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("expected relayed packet, got error: %v", err)
	}
	got := strings.TrimSpace(string(buf[:n]))
	if !strings.Contains(got, "from client") {
		t.Errorf("relayed packet = %q, want it to contain 'from client'", got)
	}

	// An uplink-sourced packet must NOT be relayed to the peer.
	uplink.Stream.Write(parsePkt(t, "UP>DST,qAR,IGATE:from upstream"), "uplink")
	_ = fake.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if n, _, err := fake.ReadFromUDP(buf); err == nil {
		t.Errorf("uplink-sourced packet should not reach peer, but got: %q", string(buf[:n]))
	}
}

// TestPeerTCPRelayAndInject verifies a TCP core peer: a client-sourced packet
// is relayed out over the TCP connection, and a packet sent by the peer over
// that connection is injected into the stream tagged as a peer source.
func TestPeerTCPRelayAndInject(t *testing.T) {
	logger.L = zap.NewNop()
	config2.Set(testConfig())
	uplink.Stream = uplink.NewDataStream(10)
	ch, unsub := uplink.Stream.Subscribe()
	defer unsub()

	// A fake remote TCP peer the manager will dial.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("fake peer listen: %v", err)
	}
	defer ln.Close()
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())

	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- c
	}()

	// Manager configured with one TCP peer pointing at the fake server. Its own
	// inbound listener binds to an ephemeral port to avoid clashing.
	m := &Manager{bindHost: "127.0.0.1", bindPort: 0, stop: make(chan struct{})}
	m.peers = []*remotePeer{{
		name: "fake", id: "SRV1", tcp: true,
		tcpAddr: net.JoinHostPort("127.0.0.1", portStr),
		ip:      net.ParseIP("127.0.0.1"),
	}}
	if err := m.start(); err != nil {
		t.Fatalf("start peer manager: %v", err)
	}
	defer m.shutdown()

	// Wait for the manager's outbound dial to be accepted.
	var peerConn net.Conn
	select {
	case peerConn = <-accepted:
	case <-time.After(3 * time.Second):
		t.Fatal("manager did not dial the TCP peer")
	}
	defer peerConn.Close()

	// Outbound: a client-sourced packet should be relayed over the TCP link.
	uplink.Stream.Write(parsePkt(t, "SRC>DST,qAR,IGATE:from client"), "N5CAL-1")
	_ = peerConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := bufio.NewReader(peerConn).ReadString('\n')
	if err != nil {
		t.Fatalf("expected relayed packet over TCP, got: %v", err)
	}
	if !strings.Contains(line, "from client") {
		t.Errorf("relayed TCP packet = %q, want it to contain 'from client'", line)
	}

	// Inbound: a packet sent by the peer over the connection is injected.
	if _, err := peerConn.Write([]byte("PSRC>DST,qAR,IGATE:from peer\r\n")); err != nil {
		t.Fatalf("peer write: %v", err)
	}
	// The stream also carries our own outbound client packet; wait for the
	// peer-sourced one specifically.
	deadline := time.After(3 * time.Second)
	for {
		select {
		case data := <-ch:
			if !strings.HasPrefix(data.Writer, WriterPrefix) {
				continue // skip the client-sourced packet we injected earlier
			}
			if data.Data.From != "PSRC" {
				t.Errorf("From = %q, want PSRC", data.Data.From)
			}
			return
		case <-deadline:
			t.Fatal("peer TCP packet not injected into stream")
		}
	}
}
