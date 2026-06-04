package listener

import (
	"net"
	"testing"
	"time"

	"github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"github.com/APRSCN/aprsgo/internal/network/uplink"
	"go.uber.org/zap"
)

// TestUDPSubmitServerEndToEnd starts the real UDP submit server, sends a
// datagram, and verifies the packet is injected with a qAU construct.
func TestUDPSubmitServerEndToEnd(t *testing.T) {
	logger.L = zap.NewNop()
	config.Set(testConfig())
	uplink.Stream = uplink.NewDataStream(10)
	ch, unsub := uplink.Stream.Subscribe()
	defer unsub()

	// A listener entry must exist for stats bookkeeping (index 0). We do not
	// reset Listeners in a defer: server goroutines may still touch it briefly
	// after Stop, and each test re-sets Listeners at its start anyway.
	Listeners = []*Listener{{Name: "udp-test", Protocol: "udp"}}

	srv := NewUDPSubmitServer(0)
	if err := srv.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("start udp server: %v", err)
	}
	defer srv.Stop()

	localAddr := srv.conn.LocalAddr().(*net.UDPAddr)

	conn, err := net.Dial("udp", localAddr.String())
	if err != nil {
		t.Fatalf("dial udp: %v", err)
	}
	defer conn.Close()

	datagram := "user TEST pass 29939 vers sw 1.0\r\nTEST>APRS,TCPIP*:>e2e udp\r\n"
	if _, err := conn.Write([]byte(datagram)); err != nil {
		t.Fatalf("write udp: %v", err)
	}

	select {
	case data := <-ch:
		found := false
		for _, hop := range data.Data.Path {
			if hop == "qAU" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected qAU in path, got %v", data.Data.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no packet injected from UDP datagram")
	}
}
