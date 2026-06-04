//go:build !linux

package listener

import (
	"errors"
	"net"
)

// listenSCTP is unavailable on non-Linux platforms.
func listenSCTP(addr string) (net.Listener, error) {
	return nil, errors.New("SCTP is only supported on Linux")
}

// sctpSupported reports whether SCTP is available on this platform.
func sctpSupported() bool { return false }
