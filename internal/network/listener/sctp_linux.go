//go:build linux

package listener

import (
	"net"

	"github.com/ishidawataru/sctp"
)

// listenSCTP opens an SCTP listener on addr (host:port). SCTP is only
// supported on Linux.
func listenSCTP(addr string) (net.Listener, error) {
	a, err := sctp.ResolveSCTPAddr("sctp", addr)
	if err != nil {
		return nil, err
	}
	ln, err := sctp.ListenSCTP("sctp", a)
	if err != nil {
		return nil, err
	}
	return ln, nil
}

// sctpSupported reports whether SCTP is available on this platform.
func sctpSupported() bool { return true }
