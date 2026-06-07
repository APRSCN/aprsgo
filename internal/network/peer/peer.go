// Package peer implements APRS-IS core-peer links: server-to-server
// connections that exchange raw APRS packet lines (no login envelope),
// identified by the remote address. Each configured peer uses either UDP
// (connectionless datagrams) or TCP (a persistent stream), and a group may mix
// both.
//
// Loop-prevention rules:
//   - Traffic from core peers is delivered to local clients.
//   - Traffic from local clients is relayed to core peers.
//   - Traffic does NOT pass between peers and upstream uplinks.
//   - A packet received from a peer is never echoed back to that peer.
package peer

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"github.com/APRSCN/aprsgo/internal/network/uplink"
	"github.com/APRSCN/aprsgo/internal/wait"
	"github.com/APRSCN/aprsutils/parser"
	"github.com/APRSCN/aprsutils/qConstruct"
	"go.uber.org/zap"
)

// WriterPrefix tags stream packets that originated from a peer, as
// "peer:<id>". Consumers use it to avoid echoing packets back to their source
// and to keep peer/uplink traffic separate. It aliases the canonical tag
// defined in the uplink package.
const WriterPrefix = uplink.WriterPeerPrefix

// maxPeerDatagram bounds a single inbound peer datagram (UDP) / read buffer.
const maxPeerDatagram = 2048

// TCP peer reconnect backoff bounds.
const (
	peerMinBackoff = 1 * time.Second
	peerMaxBackoff = 30 * time.Second
)

// remotePeer is a configured remote peer.
type remotePeer struct {
	name string
	id   string
	tcp  bool // transport is TCP (otherwise UDP)

	// UDP transport: resolved datagram address.
	udpAddr *net.UDPAddr

	// TCP transport: dial target and the set of currently-open connections
	// (an outbound dialled connection and/or an accepted inbound one). Writes
	// go to every open connection; the dedup layer absorbs any duplication.
	tcpAddr string
	ip      net.IP // resolved remote IP, for matching inbound TCP connections
	connsMu sync.Mutex
	conns   map[net.Conn]struct{}
}

// addConn registers an open TCP connection for this peer.
func (p *remotePeer) addConn(c net.Conn) {
	p.connsMu.Lock()
	if p.conns == nil {
		p.conns = make(map[net.Conn]struct{})
	}
	p.conns[c] = struct{}{}
	p.connsMu.Unlock()
}

// removeConn unregisters a closed TCP connection.
func (p *remotePeer) removeConn(c net.Conn) {
	p.connsMu.Lock()
	delete(p.conns, c)
	p.connsMu.Unlock()
}

// writeAll writes raw to every open connection for this peer.
func (p *remotePeer) writeAll(raw []byte) {
	p.connsMu.Lock()
	conns := make([]net.Conn, 0, len(p.conns))
	for c := range p.conns {
		conns = append(conns, c)
	}
	p.connsMu.Unlock()
	for _, c := range conns {
		_ = c.SetWriteDeadline(time.Now().Add(30 * time.Second))
		if _, err := c.Write(raw); err != nil {
			logger.L.Debug("Peer TCP send error", zap.String("peer", p.name), zap.Error(err))
			_ = c.Close()
			p.removeConn(c)
		}
	}
}

// Manager owns a peer group's transports (a shared UDP socket for UDP peers
// and a TCP listener for TCP peers) and the set of configured peers.
type Manager struct {
	name  string
	peers []*remotePeer

	udpConn  *net.UDPConn // nil when the group has no UDP peers
	tcpLn    net.Listener // nil when the group has no TCP peers
	bindHost string
	bindPort int

	unsub    func()
	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// managers holds every active peer-group manager (empty until Init configures
// peers). It is replaced wholesale by Init and read by List/Stop, guarded by
// managersMu.
var (
	managers   []*Manager
	managersMu sync.RWMutex
)

// Info describes a configured remote peer for status reporting.
type Info struct {
	Name string
	ID   string
	Addr string
}

// List returns the configured peers across all groups. Safe to call when no
// peer manager is running (returns nil).
func List() []Info {
	managersMu.RLock()
	defer managersMu.RUnlock()
	var out []Info
	for _, m := range managers {
		for _, p := range m.peers {
			out = append(out, Info{Name: p.name, ID: p.id, Addr: p.addrString()})
		}
	}
	return out
}

// addrString returns a human-readable address for status display.
func (p *remotePeer) addrString() string {
	if p.tcp {
		return p.tcpAddr
	}
	return p.udpAddr.String()
}

// Init creates and starts a peer manager per configured group. It is a no-op
// when no peers are configured. The legacy single-group config ('peer') and
// the 'peergroups' list are both honoured.
func Init() {
	groups := configuredGroups()
	if len(groups) == 0 {
		logger.L.Debug("No core peers configured")
		return
	}

	var started []*Manager
	for _, gc := range groups {
		m := buildManager(gc)
		if len(m.peers) == 0 {
			continue
		}
		if err := m.start(); err != nil {
			logger.L.Error("Failed to start peer group",
				zap.String("name", m.name), zap.Error(err))
			continue
		}
		started = append(started, m)
		logger.L.Info("Core peer group started",
			zap.String("name", m.name),
			zap.String("bind", fmt.Sprintf("%s:%d", m.bindHost, m.bindPort)),
			zap.Int("peers", len(m.peers)))
	}

	managersMu.Lock()
	managers = started
	managersMu.Unlock()
}

// configuredGroups returns the union of the legacy single-group config and the
// peergroups list (groups with no peers are skipped by the caller).
func configuredGroups() []config.PeerGroupConfig {
	var groups []config.PeerGroupConfig
	if legacy := config.Get().Server.Peer; len(legacy.Peers) > 0 {
		if legacy.Name == "" {
			legacy.Name = "default"
		}
		groups = append(groups, legacy)
	}
	groups = append(groups, config.Get().Server.PeerGroups...)
	return groups
}

// buildManager resolves a group's peer addresses into a Manager.
func buildManager(gc config.PeerGroupConfig) *Manager {
	m := &Manager{name: gc.Name, bindHost: gc.Host, bindPort: gc.Port, stop: make(chan struct{})}
	for _, p := range gc.Peers {
		hostport := fmt.Sprintf("%s:%d", p.Host, p.Port)
		if strings.EqualFold(p.Protocol, "tcp") {
			rp := &remotePeer{name: p.Name, id: p.ID, tcp: true, tcpAddr: hostport}
			if ips, err := net.LookupIP(p.Host); err == nil && len(ips) > 0 {
				rp.ip = ips[0]
			}
			m.peers = append(m.peers, rp)
			continue
		}
		addr, err := net.ResolveUDPAddr("udp", hostport)
		if err != nil {
			logger.L.Error("Invalid peer address, skipping",
				zap.String("name", p.Name), zap.Error(err))
			continue
		}
		m.peers = append(m.peers, &remotePeer{name: p.Name, id: p.ID, udpAddr: addr})
	}
	return m
}

// hasUDP / hasTCP report whether the group includes any peer of that transport.
func (m *Manager) hasUDP() bool {
	for _, p := range m.peers {
		if !p.tcp {
			return true
		}
	}
	return false
}

func (m *Manager) hasTCP() bool {
	for _, p := range m.peers {
		if p.tcp {
			return true
		}
	}
	return false
}

// start opens the needed transports and launches the receive/dial/send loops.
func (m *Manager) start() error {
	bind := fmt.Sprintf("%s:%d", m.bindHost, m.bindPort)

	if m.hasUDP() {
		udpAddr, err := net.ResolveUDPAddr("udp", bind)
		if err != nil {
			return err
		}
		conn, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			return err
		}
		m.udpConn = conn
		m.wg.Add(1)
		go m.udpReceiveLoop()
	}

	if m.hasTCP() {
		ln, err := net.Listen("tcp", bind)
		if err != nil {
			if m.udpConn != nil {
				_ = m.udpConn.Close()
			}
			return err
		}
		m.tcpLn = ln
		m.wg.Add(1)
		go m.tcpAcceptLoop()
		// Dial each TCP peer (outbound half of the bidirectional link).
		for _, p := range m.peers {
			if p.tcp {
				m.wg.Add(1)
				go m.tcpDialLoop(p)
			}
		}
	}

	// Outbound: relay the distribution stream to peers.
	ch, unsub := uplink.Stream.Subscribe()
	m.unsub = unsub
	m.wg.Add(1)
	go m.sendLoop(ch)

	return nil
}

// Stop shuts every peer-group manager down.
func Stop() {
	managersMu.Lock()
	current := managers
	managers = nil
	managersMu.Unlock()
	for _, m := range current {
		m.shutdown()
	}
}

// Reload restarts the peer managers from the (already reloaded) configuration,
// so changed peer groups take effect on SIGHUP.
func Reload() {
	Stop()
	Init()
	logger.L.Info("Core peers reloaded")
}

func (m *Manager) shutdown() {
	m.stopOnce.Do(func() { close(m.stop) })
	if m.udpConn != nil {
		_ = m.udpConn.Close()
	}
	if m.tcpLn != nil {
		_ = m.tcpLn.Close()
	}
	// Close any open TCP peer connections so their read loops unblock.
	for _, p := range m.peers {
		p.connsMu.Lock()
		for c := range p.conns {
			_ = c.Close()
		}
		p.connsMu.Unlock()
	}
	if m.unsub != nil {
		m.unsub()
	}
	m.wg.Wait()
}

// udpReceiveLoop reads datagrams and injects packets from recognised peers.
func (m *Manager) udpReceiveLoop() {
	defer m.wg.Done()
	buf := make([]byte, maxPeerDatagram)
	for {
		n, remote, err := m.udpConn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-m.stop:
				return
			default:
				logger.L.Debug("Peer UDP read error", zap.Error(err))
				continue
			}
		}
		src := m.matchUDPPeer(remote)
		if src == nil {
			logger.L.Debug("Peer datagram from unknown source", zap.String("remote", remote.String()))
			continue
		}
		m.handlePayload(string(buf[:n]), src)
	}
}

// tcpAcceptLoop accepts inbound TCP peer connections, matching them to a
// configured peer by source IP.
func (m *Manager) tcpAcceptLoop() {
	defer m.wg.Done()
	for {
		conn, err := m.tcpLn.Accept()
		if err != nil {
			select {
			case <-m.stop:
				return
			default:
				logger.L.Debug("Peer TCP accept error", zap.Error(err))
				continue
			}
		}
		src := m.matchTCPPeer(conn.RemoteAddr())
		if src == nil {
			logger.L.Debug("Peer TCP connection from unknown source",
				zap.String("remote", conn.RemoteAddr().String()))
			_ = conn.Close()
			continue
		}
		src.addConn(conn)
		logger.L.Info("Core peer connected (inbound)",
			zap.String("peer", src.name), zap.String("remote", conn.RemoteAddr().String()))
		m.wg.Add(1)
		go m.tcpReadLoop(conn, src)
	}
}

// tcpDialLoop maintains an outbound TCP connection to a peer, reconnecting with
// exponential backoff when it drops.
func (m *Manager) tcpDialLoop(p *remotePeer) {
	defer m.wg.Done()
	backoff := peerMinBackoff
	for {
		select {
		case <-m.stop:
			return
		default:
		}

		conn, err := net.DialTimeout("tcp", p.tcpAddr, 10*time.Second)
		if err != nil {
			if !wait.SleepOrStop(m.stop, backoff) {
				return
			}
			backoff *= 2
			if backoff > peerMaxBackoff {
				backoff = peerMaxBackoff
			}
			continue
		}
		backoff = peerMinBackoff
		p.addConn(conn)
		logger.L.Info("Core peer connected (outbound)",
			zap.String("peer", p.name), zap.String("remote", p.tcpAddr))

		// Read on the outbound connection too (full-duplex); blocks until it
		// drops, then we reconnect.
		m.tcpReadConn(conn, p)

		select {
		case <-m.stop:
			return
		default:
		}
		if !wait.SleepOrStop(m.stop, peerMinBackoff) {
			return
		}
	}
}

// tcpReadLoop is the wg-tracked entry point for an accepted inbound connection.
func (m *Manager) tcpReadLoop(conn net.Conn, src *remotePeer) {
	defer m.wg.Done()
	m.tcpReadConn(conn, src)
}

// tcpReadConn reads APRS lines from a TCP peer connection until it closes,
// injecting each into the stream. It unregisters and closes the connection on
// return.
func (m *Manager) tcpReadConn(conn net.Conn, src *remotePeer) {
	defer func() {
		src.removeConn(conn)
		_ = conn.Close()
	}()
	r := bufio.NewReaderSize(conn, maxPeerDatagram)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(strings.TrimRight(line, "\r\n"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m.injectFromPeer(line, src)
	}
}

// matchUDPPeer returns the UDP peer matching the remote address, or nil.
func (m *Manager) matchUDPPeer(remote *net.UDPAddr) *remotePeer {
	for _, p := range m.peers {
		if !p.tcp && p.udpAddr.IP.Equal(remote.IP) && p.udpAddr.Port == remote.Port {
			return p
		}
	}
	return nil
}

// matchTCPPeer returns the TCP peer whose resolved IP matches the inbound
// connection's source IP, or nil. The source port is ephemeral and ignored.
func (m *Manager) matchTCPPeer(remote net.Addr) *remotePeer {
	host, _, err := net.SplitHostPort(remote.String())
	if err != nil {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	for _, p := range m.peers {
		if p.tcp && p.ip != nil && p.ip.Equal(ip) {
			return p
		}
	}
	return nil
}

// handlePayload processes one datagram (which may contain multiple lines) from
// a known UDP peer.
func (m *Manager) handlePayload(payload string, src *remotePeer) {
	for _, line := range strings.Split(payload, "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m.injectFromPeer(line, src)
	}
}

// injectFromPeer q-processes and injects a single packet received from a peer.
func (m *Manager) injectFromPeer(packet string, src *remotePeer) {
	parsed, _ := parser.Parse(packet, parser.WithDisableToCallsignValidate())
	if parsed.To == "" {
		return
	}

	// Peers are treated as outbound servers for q-construct purposes: an
	// existing q construct is preserved, qAI traces get the server appended,
	// and loop detection applies.
	qConfig := &qConstruct.QConfig{
		ServerLogin:    config.Get().Server.ID,
		ClientLogin:    src.id,
		ConnectionType: qConstruct.ConnectionOutboundServer,
		IsVerified:     true,
		RemoteIP:       src.remoteIP(),
	}
	result, err := qConstruct.QConstruct(parsed, qConfig)
	if err != nil || result.ShouldDrop || result.IsLoop {
		return
	}

	packet, err = qConstruct.Replace(packet, parsed.To, result.Path)
	if err != nil {
		return
	}
	parsed, err = parser.Parse(packet, parser.WithDisableToCallsignValidate())
	if err != nil {
		return
	}

	uplink.Stream.Write(parsed, WriterPrefix+src.id)
}

// remoteIP returns the peer's remote IP string for q-construct (IP->hex).
func (p *remotePeer) remoteIP() string {
	if p.tcp {
		if p.ip != nil {
			return p.ip.String()
		}
		return ""
	}
	return p.udpAddr.IP.String()
}

// sendLoop relays stream packets to all peers, honouring loop-prevention rules.
func (m *Manager) sendLoop(ch <-chan uplink.StreamData) {
	defer m.wg.Done()
	for {
		select {
		case <-m.stop:
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			m.relay(data)
		}
	}
}

// relay forwards a packet to peers, skipping packets that came from the uplink
// (no upstream<->peer cross-feed) and skipping the peer that sent it.
func (m *Manager) relay(data uplink.StreamData) {
	if data.Writer == uplink.WriterUplink {
		return // Do not forward upstream traffic to peers.
	}
	var fromPeerID string
	if strings.HasPrefix(data.Writer, WriterPrefix) {
		fromPeerID = strings.TrimPrefix(data.Writer, WriterPrefix)
	}

	raw := []byte(data.Data.Raw + "\r\n")
	for _, p := range m.peers {
		if fromPeerID != "" && p.id == fromPeerID {
			continue // never echo back to the source peer
		}
		if p.tcp {
			p.writeAll(raw)
			continue
		}
		if _, err := m.udpConn.WriteToUDP(raw, p.udpAddr); err != nil {
			logger.L.Debug("Peer UDP send error",
				zap.String("peer", p.name), zap.Error(err))
		}
	}
}
