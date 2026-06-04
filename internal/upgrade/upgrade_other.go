//go:build !unix

package upgrade

import (
	"errors"
	"net"
)

// On platforms without file-descriptor passing, sockets are always bound fresh
// and live upgrade is unavailable.

func supported() bool { return false }

func listenTCP(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

func listenUDP(addr string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	return net.ListenUDP("udp", udpAddr)
}

func perform() (int, error) {
	return 0, errors.New("live upgrade is not supported on this platform")
}
