// Package upgrade implements zero-downtime restarts by handing the listening
// sockets to a freshly-exec'd copy of the binary. The child inherits the bound
// listener file descriptors and starts accepting on them immediately, so the
// data-plane ports never close; the parent then drains its existing
// connections and exits.
//
// Listeners participate by creating their sockets through ListenTCP / ListenUDP
// instead of binding directly: on a normal start these bind fresh and register
// the socket for later handoff; after an upgrade they transparently adopt the
// inherited socket matching their address.
//
// Socket inheritance relies on passing file descriptors to a child process,
// which is only available on Unix-like systems. On other platforms an upgrade
// request returns an error and ListenTCP / ListenUDP simply bind fresh.
package upgrade

import "net"

// envFDList is the environment variable carrying the inherited-socket map from
// parent to child: a comma-separated list of "key" entries, in the same order
// as the extra file descriptors (which start at fd 3 in the child).
const envFDList = "APRSGO_UPGRADE_FDS"

func tcpKey(addr string) string { return "tcp:" + addr }
func udpKey(addr string) string { return "udp:" + addr }

// ListenTCP returns a TCP listener for addr, adopting an inherited socket if
// one was passed by a parent process during an upgrade, otherwise binding
// fresh. The returned listener is registered for a future handoff.
func ListenTCP(addr string) (net.Listener, error) {
	return listenTCP(addr)
}

// ListenUDP returns a UDP packet connection for addr, adopting an inherited
// socket if available, otherwise binding fresh, and registers it for handoff.
func ListenUDP(addr string) (*net.UDPConn, error) {
	return listenUDP(addr)
}

// Supported reports whether socket handoff is available on this platform.
func Supported() bool { return supported() }

// Perform spawns a child process that inherits the registered listening
// sockets and starts serving on them. It returns the started child's PID. The
// caller should then gracefully drain and exit the current process. On
// platforms without FD passing it returns an error.
func Perform() (pid int, err error) { return perform() }
