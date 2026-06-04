//go:build unix

package upgrade

import (
	"net"
	"os"
	"sync"
	"testing"
)

// resetState clears the package registries between tests.
func resetState(t *testing.T) {
	t.Helper()
	mu.Lock()
	registered = make(map[string]*os.File)
	order = nil
	mu.Unlock()
	inheritOnce = sync.Once{}
	inheritedMap = nil
	_ = os.Unsetenv(envFDList)
}

func TestListenTCPFreshBindAndRegister(t *testing.T) {
	resetState(t)

	const addr = "127.0.0.1:0"
	l, err := ListenTCP(addr)
	if err != nil {
		t.Fatalf("ListenTCP: %v", err)
	}
	defer l.Close()

	// Registration is keyed by the configured address string (stable across
	// parent and child), not the resolved port.
	key := tcpKey(addr)
	mu.Lock()
	_, ok := registered[key]
	n := len(order)
	mu.Unlock()
	if !ok {
		t.Errorf("listener was not registered under %q", key)
	}
	if n != 1 {
		t.Errorf("registration order length = %d, want 1", n)
	}
}

func TestListenUDPFreshBindAndRegister(t *testing.T) {
	resetState(t)

	const addr = "127.0.0.1:0"
	c, err := ListenUDP(addr)
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	defer c.Close()

	key := udpKey(addr)
	mu.Lock()
	_, ok := registered[key]
	mu.Unlock()
	if !ok {
		t.Errorf("UDP listener was not registered under %q", key)
	}
}

// A TCP socket presented as an inherited file descriptor must be adopted by
// ListenTCP for the matching address instead of binding fresh.
func TestListenTCPAdoptsInherited(t *testing.T) {
	resetState(t)

	// Bind a real listener to stand in for the parent's socket.
	parent, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("parent listen: %v", err)
	}
	defer parent.Close()
	addr := parent.Addr().String()

	tl := parent.(*net.TCPListener)
	f, err := tl.File()
	if err != nil {
		t.Fatalf("listener File: %v", err)
	}
	defer f.Close()

	// Present the file as the single inherited socket: install it directly into
	// the inherited map under the matching key and mark the one-time parse as
	// already done so adoptListener uses our map.
	inheritedMap = map[string]*os.File{tcpKey(addr): f}
	inheritOnce.Do(func() {})

	got, err := ListenTCP(addr)
	if err != nil {
		t.Fatalf("ListenTCP adopt: %v", err)
	}
	defer got.Close()

	if got.Addr().String() != addr {
		t.Errorf("adopted listener addr = %q, want %q", got.Addr().String(), addr)
	}
}
