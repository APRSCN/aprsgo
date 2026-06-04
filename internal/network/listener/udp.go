package listener

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/internal/upgrade"
	"go.uber.org/zap"
)

// maxUDPDatagram is the maximum UDP submit datagram size we accept. APRS
// packets are short; this bounds memory per read.
const maxUDPDatagram = 2048

// UDPSubmitServer listens for connectionless APRS-IS "UDP submit" datagrams.
//
// Each datagram contains a login envelope followed by one or more packets:
//
//	user CALL pass CODE vers SW VER\r\n
//	PACKET\r\n
//
// Authenticated packets are injected with a qAU construct (see ProcessSubmit /
// SubmitUDP).
type UDPSubmitServer struct {
	conn  *net.UDPConn
	index int
	stop  chan struct{}
	wg    sync.WaitGroup
	stats model.Counters
}

// NewUDPSubmitServer creates a UDP submit server for the listener at index.
func NewUDPSubmitServer(index int) *UDPSubmitServer {
	return &UDPSubmitServer{
		index: index,
		stop:  make(chan struct{}),
	}
}

// Start begins listening on addr (host:port).
func (s *UDPSubmitServer) Start(addr string) error {
	conn, err := upgrade.ListenUDP(addr)
	if err != nil {
		return err
	}
	s.conn = conn
	logger.L.Info(fmt.Sprintf("APRS UDP submit listening on %s", addr))
	s.wg.Add(2)
	go s.serve()
	go s.updateStats()
	return nil
}

// Stop closes the UDP socket, ends the serve/stats loops and waits for them.
func (s *UDPSubmitServer) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
	s.wg.Wait()
}

// serve reads and processes datagrams until stopped.
func (s *UDPSubmitServer) serve() {
	defer s.wg.Done()
	buf := make([]byte, maxUDPDatagram)
	for {
		n, remote, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-s.stop:
				return
			default:
				logger.L.Debug("UDP submit read error", zap.Error(err))
				continue
			}
		}
		s.handleDatagram(string(buf[:n]), remote)
	}
}

// updateStats recomputes the per-second rates once a second and publishes a
// snapshot to the listener entry, mirroring the TCP server so the status page
// shows non-zero UDP submit rates.
func (s *UDPSubmitServer) updateStats() {
	defer s.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.stats.UpdateRates()
			if l := listenerAt(s.index); l != nil {
				l.SetStats(s.stats.Snapshot())
			}
		}
	}
}

// handleDatagram parses one datagram's envelope and submits its packets.
func (s *UDPSubmitServer) handleDatagram(payload string, remote *net.UDPAddr) {
	// Access-control: drop datagrams from addresses the ACL rejects.
	if l := listenerAt(s.index); l != nil && !l.acl.AllowAddr(remote.AddrPort().Addr()) {
		logger.L.Debug("UDP submit rejected by ACL", zap.String("remote", remote.String()))
		return
	}

	call, verified, packets, ok := parseSubmitEnvelope(payload)
	if !ok {
		logger.L.Debug("UDP submit: missing/invalid login envelope",
			zap.String("remote", remote.String()))
		s.bumpStats(0, 0, 1)
		return
	}
	if !verified {
		// Unverified UDP submit is rejected (no qAX path for connectionless).
		logger.L.Debug("UDP submit: unverified login",
			zap.String("remote", remote.String()),
			zap.String("callsign", call))
		s.bumpStats(0, 0, 1)
		return
	}

	for _, pkt := range packets {
		if err := ProcessSubmit(call, true, pkt, SubmitUDP); err != nil {
			switch {
			case errors.Is(err, ErrSubmitDuplicate):
				s.bumpStats(1, 1, 0)
			case errors.Is(err, ErrSubmitQDrop):
				s.bumpStats(1, 0, 0)
			default:
				s.bumpStats(1, 0, 1)
			}
			continue
		}
		s.bumpStats(1, 0, 0)
	}
}

// bumpStats updates the UDP submit cumulative counters (atomic). The derived
// per-second rates and the published snapshot are refreshed once a second by
// updateStats.
func (s *UDPSubmitServer) bumpStats(received, dups, errs uint64) {
	s.stats.AddReceivedPackets(received)
	s.stats.AddReceivedDups(dups)
	s.stats.AddReceivedErrors(errs)
}
