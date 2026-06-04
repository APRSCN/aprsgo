package listener

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"github.com/APRSCN/aprsgo/internal/network/uplink"
	"github.com/APRSCN/aprsutils"
	"github.com/APRSCN/aprsutils/client"
	"go.uber.org/zap"
)

// startTestTCPServer spins up a TCPAPRSServer on a free local port and returns
// it plus the bound address. It also wires a single Listeners entry so stats
// bookkeeping works. Each test sets Listeners at start; we never reset it in a
// defer while server goroutines are still draining (that would race the global).
func startTestTCPServer(t *testing.T, mode client.Mode) (*TCPAPRSServer, string) {
	t.Helper()
	srv := NewTCPAPRSServer(mode, 0)
	Listeners = []*Listener{{Name: "test", Protocol: "tcp", s: srv}}
	if err := srv.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("start server: %v", err)
	}
	return srv, srv.listener.Addr().String()
}

// readLine reads one CRLF/LF-terminated line with a deadline.
func readLine(t *testing.T, r *bufio.Reader, conn net.Conn) string {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		t.Fatalf("read line: %v", err)
	}
	return strings.TrimSpace(line)
}

// TestTCPLoginAndForward exercises the full TCP path: connect, login (verified),
// send a packet, and verify it is injected into the distribution stream with a
// qAC construct.
func TestTCPLoginAndForward(t *testing.T) {
	logger.L = zap.NewNop()
	config.Set(testConfig())
	uplink.Stream = uplink.NewDataStream(10)
	ch, unsub := uplink.Stream.Subscribe()
	defer unsub()

	srv, addr := startTestTCPServer(t, client.Fullfeed)
	defer srv.Stop()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	r := bufio.NewReader(conn)

	// Server greeting (a comment line beginning with '#').
	greet := readLine(t, r, conn)
	if !strings.HasPrefix(greet, "#") {
		t.Fatalf("expected greeting comment, got %q", greet)
	}

	// Login as TEST1 (compute its passcode -> verified).
	fmt.Fprintf(conn, "user TEST1 pass %d vers test 1.0\r\n", aprsutils.Passcode("TEST1"))

	resp := readLine(t, r, conn)
	if !strings.Contains(resp, "verified") {
		t.Fatalf("expected verified logresp, got %q", resp)
	}

	// Send a packet originating from the client; should become TCPIP*,qAC,SERVER.
	fmt.Fprintf(conn, "TEST1>APRS,WIDE1-1:>integration test\r\n")

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
		if data.Writer != "TEST1" {
			t.Errorf("writer = %q, want TEST1", data.Writer)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("packet not injected into stream")
	}
}

// TestTCPUnverifiedRejected verifies an unverified client cannot inject data.
func TestTCPUnverifiedRejected(t *testing.T) {
	logger.L = zap.NewNop()
	config.Set(testConfig())
	uplink.Stream = uplink.NewDataStream(10)
	ch, unsub := uplink.Stream.Subscribe()
	defer unsub()

	srv, addr := startTestTCPServer(t, client.Fullfeed)
	defer srv.Stop()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	_ = readLine(t, r, conn) // greeting

	// Wrong passcode -> unverified.
	fmt.Fprintf(conn, "user N0CALL pass 1 vers test 1.0\r\n")
	resp := readLine(t, r, conn)
	if !strings.Contains(resp, "unverified") {
		t.Fatalf("expected unverified logresp, got %q", resp)
	}

	// Data from an unverified client must be rejected (server replies, no inject).
	fmt.Fprintf(conn, "N0CALL>APRS:>should be rejected\r\n")
	select {
	case data := <-ch:
		t.Errorf("unverified client packet should not be injected, got %v", data.Data.Raw)
	case <-time.After(500 * time.Millisecond):
		// expected: nothing injected
	}
}

// TestTCPProbeRejected verifies an HTTP probe on the APRS port is dropped
// without being treated as a client.
func TestTCPProbeRejected(t *testing.T) {
	logger.L = zap.NewNop()
	config.Set(testConfig())
	uplink.Stream = uplink.NewDataStream(10)

	srv, addr := startTestTCPServer(t, client.Fullfeed)
	defer srv.Stop()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	_ = readLine(t, r, conn) // greeting

	// Send an HTTP request line; the server should close the connection.
	fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: x\r\n\r\n")

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64)
	if _, err := r.Read(buf); err == nil {
		// A read may return EOF (connection closed) which is the expected path.
		// Any successful non-EOF read of additional data is unexpected here, but
		// not strictly an error; the key behaviour is the connection closes.
	}
	// Verify the connection is closed by attempting another read.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := r.Read(buf); err == nil {
		t.Error("expected connection to be closed after HTTP probe")
	}
}

// TestKickOldSameCallsignConcurrent logs in several clients with the same
// callsign concurrently. handleLogin writes c.callSign under c.mu and kickOld
// reads other clients' callSign; this is a regression guard for the data race
// where kickOld read callSign without holding the target client's c.mu. It is
// meaningful only under `go test -race`.
func TestKickOldSameCallsignConcurrent(t *testing.T) {
	logger.L = zap.NewNop()
	config.Set(testConfig())
	uplink.Stream = uplink.NewDataStream(10)

	srv, addr := startTestTCPServer(t, client.Fullfeed)
	defer srv.Stop()

	const call = "TEST1"
	pass := aprsutils.Passcode(call)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				return
			}
			defer conn.Close()
			r := bufio.NewReader(conn)
			_, _ = r.ReadString('\n') // greeting
			fmt.Fprintf(conn, "user %s pass %d vers test 1.0\r\n", call, pass)
			_, _ = r.ReadString('\n') // logresp
			// Stay connected briefly so concurrent logins overlap in kickOld.
			time.Sleep(50 * time.Millisecond)
		}()
	}
	wg.Wait()
	// Give kickOld goroutines a moment to finish before the server stops.
	time.Sleep(100 * time.Millisecond)
}
