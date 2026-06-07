package listener

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"github.com/APRSCN/aprsgo/internal/meta"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/internal/network/uplink"
	"github.com/APRSCN/aprsgo/internal/pkg/historydb"
	"github.com/APRSCN/aprsgo/internal/security"
	"github.com/APRSCN/aprsgo/internal/upgrade"
	"github.com/APRSCN/aprsutils"
	"github.com/APRSCN/aprsutils/client"
	"github.com/APRSCN/aprsutils/filter"
	"github.com/APRSCN/aprsutils/parser"
	"github.com/APRSCN/aprsutils/qConstruct"
	"go.uber.org/zap"
)

// serverCommandFilter is the in-band login/comment keyword that introduces a
// filter specification ("... filter <spec>"). It also delimits the end of a
// preceding filter spec when it recurs.
const serverCommandFilter = "filter"

// Built-in timeout defaults, used when the corresponding config value is 0.
//
// The client idle timeout is intentionally long: liveness is detected by TCP
// keepalive (see applyKeepAlive) plus the periodic application keepalive, not
// by a short input-idle window. A short window would disconnect healthy
// stations that simply transmit infrequently (e.g. a phone app running in the
// background). Override via server.client_timeout in the config.
const (
	defaultLoginTimeout  = 30 * time.Second
	defaultClientTimeout = 48 * time.Hour
)

// Retention windows for the per-client message-routing state.
const (
	// heardRetention is how long a station stays in a client's heard list
	// (drives message routing for stations the client has recently received).
	heardRetention = 3 * time.Hour
	// courtesyRetention is how long a message originator stays eligible for a
	// courtesy position after a message to it was delivered to the client.
	courtesyRetention = 30 * time.Minute
)

// TCP keepalive parameters applied to accepted client sockets: start probing
// after the socket has been idle for keepAliveIdle, probe every
// keepAliveInterval, and drop the link after keepAliveCount failed probes.
const (
	keepAliveIdle     = 10 * time.Minute
	keepAliveInterval = 20 * time.Second
	keepAliveCount    = 3
)

// loginTimeoutDur returns the configured login timeout, or the default.
func loginTimeoutDur() time.Duration {
	if s := config.Get().Server.LoginTimeout; s > 0 {
		return time.Duration(s) * time.Second
	}
	return defaultLoginTimeout
}

// clientTimeoutDur returns the configured client idle timeout, or the default.
func clientTimeoutDur() time.Duration {
	if s := config.Get().Server.ClientTimeout; s > 0 {
		return time.Duration(s) * time.Second
	}
	return defaultClientTimeout
}

// Global statistics for all servers (atomic counters; lock-free increments).
var globalStats model.Counters

// GlobalStats returns a snapshot of the process-wide packet/byte counters.
func GlobalStats() model.Statistics { return globalStats.Snapshot() }

// GlobalClientCount returns the number of currently connected TCP clients.
func GlobalClientCount() int { return int(globalClients.Load()) }

// TCPAPRSClient provides a struct for APRS client connection
type TCPAPRSClient struct {
	// Connection related fields
	conn        net.Conn
	mu          sync.Mutex
	unsubscribe func()
	dataCh      <-chan uplink.StreamData

	// Client identification and status
	callSign   string
	verified   bool
	loggedIn   bool
	uptime     time.Time
	lastActive time.Time
	software   string
	version    string
	mode       client.Mode
	filter     string
	// compiledFilter is the client's compiled per-connection filter (igate
	// mode). It is rebuilt whenever the filter string changes (login or a
	// runtime "#filter" command).
	compiledFilter *filter.Filter

	// Server reference and duplicate checking
	server *TCPAPRSServer
	dup    *historydb.DupeChecker

	// heard is the set of distinct source callsigns this client has received,
	// each with a last-heard timestamp. It drives message routing (deliver a
	// message addressed to a station this client has recently heard) and the
	// MsgRcpts count.
	heard *historydb.HeardList

	// courtesy tracks source stations that recently originated a message
	// delivered to this client. When such a station's next position/object/
	// item packet arrives, it is delivered once (a "courtesy position") so the
	// client sees where the correspondent is, then the entry is consumed.
	courtesy *historydb.HeardList

	// dupefeed marks a client connected to a dupefeed port: it additionally
	// receives packets that were detected as duplicates.
	dupefeed bool

	// Asynchronous output queue. Send enqueues onto sendCh; writeLoop drains it
	// to the socket. outQBytes tracks the number of bytes currently queued
	// (reported as OutQ). This decouples the broadcast goroutine from slow
	// clients and gives a real backlog metric.
	sendCh    chan []byte
	outQBytes atomic.Int64
	sendOnce  sync.Once
	// closed is set (once) by Close before sendCh is closed. Send checks it to
	// avoid enqueueing after shutdown; a deferred recover additionally guards
	// the unavoidable race window between the check and the channel send.
	closed atomic.Bool

	// Statistics (atomic counters)
	stats *model.Counters

	// lastTX is the unix-nano time of the most recent line queued for delivery
	// to this client (0 = never). Read by the status updater from another
	// goroutine, so it is atomic.
	lastTX atomic.Int64

	// msgRcpts counts text messages actually delivered to this client (i.e.
	// addressed to its login or to a station it has heard). It is the
	// message-recipient count shown in the status page, distinct from the size
	// of the heard set.
	msgRcpts atomic.Int64

	// Heartbeat management
	heartbeatStopChan chan struct{}
	heartbeatMutex    sync.Mutex
}

// outQCap is the per-client output queue capacity (number of pending lines).
const outQCap = 1024

// Send queues a line for asynchronous delivery to the client. It returns an
// error if the connection is closed or the output queue is full (a sign of a
// slow/stuck client). It takes no lock on the hot path: delivery readiness is
// expressed entirely through sendCh (the closed atomic flag plus a deferred
// recover handle the shutdown race so a send on a closed channel can never
// panic). Statistics use atomic counters and require no lock.
func (c *TCPAPRSClient) Send(data string) (err error) {
	if c.closed.Load() {
		return fmt.Errorf("connection closed")
	}

	// Guard the narrow window where Close closes sendCh after the check above:
	// a send on a closed channel panics, which we turn into an error.
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("connection closed")
		}
	}()

	line := append([]byte(data), '\n')

	select {
	case c.sendCh <- line:
		c.outQBytes.Add(int64(len(line)))
		// Account for sent bytes/packets at enqueue time (matches prior
		// behaviour where Send was the accounting point).
		packetSize := uint64(len(data))
		c.stats.AddSentBytes(packetSize)
		c.server.updateServerSendStats(1, packetSize)
		c.lastTX.Store(time.Now().UnixNano())
		return nil
	default:
		// Queue full: the client cannot keep up. Drop the line and report it.
		return fmt.Errorf("output queue full")
	}
}

// startWriter launches the goroutine that drains the output queue to the
// socket. Called once after the connection is established.
func (c *TCPAPRSClient) startWriter() {
	go func() {
		for line := range c.sendCh {
			c.mu.Lock()
			conn := c.conn
			c.mu.Unlock()
			if conn == nil {
				c.outQBytes.Add(-int64(len(line)))
				continue
			}
			_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			_, err := conn.Write(line)
			c.outQBytes.Add(-int64(len(line)))
			if err != nil {
				// Write failed (slow/dead client). Close the connection; the
				// read loop will then tear down the client.
				c.Close()
				return
			}
		}
	}()
}

// Close connection of client
func (c *TCPAPRSClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Stop heartbeat
	c.stopHeartbeat()

	// Stop the writer goroutine exactly once. Mark closed first so Send stops
	// enqueueing before the channel is closed.
	c.sendOnce.Do(func() {
		c.closed.Store(true)
		if c.sendCh != nil {
			close(c.sendCh)
		}
	})

	if c.unsubscribe != nil {
		c.unsubscribe()
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

// startHeartbeat starts a heartbeat ticker that is completely managed within the closure
func (c *TCPAPRSClient) startHeartbeat() {
	c.heartbeatMutex.Lock()
	defer c.heartbeatMutex.Unlock()

	// Don't start if already stopped
	if c.heartbeatStopChan != nil {
		return
	}

	c.heartbeatStopChan = make(chan struct{})
	// Capture the channel locally so the goroutine never reads the (mutable)
	// struct field, which stopHeartbeat may nil out concurrently.
	stopCh := c.heartbeatStopChan

	go func() {
		// Create ticker inside the goroutine
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Safely check if we should send heartbeat
				c.mu.Lock()
				shouldSend := c.conn != nil && time.Since(c.lastActive) >= 20*time.Second
				c.mu.Unlock()

				if shouldSend {
					c.sendHeartbeat()
				}

			case <-stopCh:
				return
			}
		}
	}()
}

// stopHeartbeat stops the heartbeat goroutine
func (c *TCPAPRSClient) stopHeartbeat() {
	c.heartbeatMutex.Lock()
	defer c.heartbeatMutex.Unlock()

	if c.heartbeatStopChan != nil {
		select {
		case <-c.heartbeatStopChan:
			// Already closed
		default:
			close(c.heartbeatStopChan)
		}
		c.heartbeatStopChan = nil
	}
}

// sendHeartbeat enqueues a single keepalive line. It does not retry: the
// output queue already absorbs transient backpressure, and a genuinely dead
// connection is detected by the writeLoop's write error, which closes the
// client. A full queue here is transient and simply skipped until the next
// tick.
func (c *TCPAPRSClient) sendHeartbeat() {
	if err := c.Send(fmt.Sprintf(
		"# %s-%s %s %s %s",
		meta.ENName, meta.Nickname, meta.Version,
		time.Now().Format(time.RFC1123),
		config.Get().Server.ID,
	)); err != nil {
		logger.L.Debug("Heartbeat enqueue skipped",
			zap.String("callsign", c.callSign), zap.Error(err))
	}
}

// handleUplinkData sends data to client from uplink stream
func (c *TCPAPRSClient) handleUplinkData() {
	for data := range c.dataCh {
		// Snapshot the delivery-relevant client state under the lock, then
		// release it before doing the (lock-free) filter match and enqueue, so
		// a slow filter/full queue never holds up other users of c.mu.
		c.mu.Lock()
		snap := deliverState{
			loggedIn:       c.loggedIn,
			connected:      c.conn != nil,
			callSign:       c.callSign,
			mode:           c.mode,
			dupefeed:       c.dupefeed,
			compiledFilter: c.compiledFilter,
		}
		c.mu.Unlock()

		if !(snap.loggedIn && snap.connected && data.Writer != snap.callSign) {
			continue
		}

		// Duplicate packets are only delivered to dupefeed ports (prefixed so
		// the receiver can tell them apart); everyone else ignores them.
		if data.Dupe {
			if snap.dupefeed {
				_ = c.Send("dup " + data.Data.Raw)
				c.stats.AddSentPackets(1)
			}
			continue
		}

		switch snap.mode {
		case client.Fullfeed:
			_ = c.Send(data.Data.Raw)
			c.stats.AddSentPackets(1)
		case client.IGate:
			if c.shouldDeliver(snap, data.Data) {
				_ = c.Send(data.Data.Raw)
				c.stats.AddSentPackets(1)
			}
		}
	}
}

// deliverState is an immutable snapshot of the client fields needed to decide
// whether to deliver a packet, taken under c.mu so the subsequent (lock-free)
// match and enqueue do not touch the mutable client state.
type deliverState struct {
	loggedIn       bool
	connected      bool
	callSign       string
	mode           client.Mode
	dupefeed       bool
	compiledFilter *filter.Filter
}

// shouldDeliver decides whether an igate-mode client should receive a packet.
// Beyond the configured filter it honours two message-related rules,
// independent of the filter:
//   - A message addressed to the client itself or to a station it has recently
//     heard is delivered; the message's source is then remembered so the
//     client can be shown where that correspondent is.
//   - The next position/object/item from a remembered correspondent is
//     delivered once (a courtesy position) and the entry consumed.
//
// It operates on a snapshot plus the synchronised heard/courtesy lists, so it
// needs no lock.
func (c *TCPAPRSClient) shouldDeliver(snap deliverState, pkt parser.Parsed) bool {
	if c.messageRouted(snap, pkt) {
		// A text message was routed to this client; count it as a recipient
		// delivery (the status MsgRcpts figure).
		c.msgRcpts.Add(1)
		// Remember the correspondent so a follow-up position is passed through.
		if c.courtesy != nil && pkt.From != "" {
			c.courtesy.Add(pkt.From)
		}
		return true
	}
	if c.passesFilter(snap, pkt) {
		return true
	}
	// Courtesy position: a single position/object/item from a station that
	// recently messaged this client, even if the filter would not pass it.
	if pkt.PacketType.Has(parser.TypePosition|parser.TypeObject|parser.TypeItem) &&
		c.courtesy != nil && pkt.From != "" && c.courtesy.Take(pkt.From) {
		return true
	}
	return false
}

// messageRouted reports whether pkt is a message whose addressee is either
// this client's own login or a station it has recently heard, in which case
// the message should be delivered regardless of the client's filter.
func (c *TCPAPRSClient) messageRouted(snap deliverState, pkt parser.Parsed) bool {
	if !pkt.PacketType.Has(parser.TypeMessage) || pkt.Addressee == "" {
		return false
	}
	addr := strings.ToUpper(strings.TrimSpace(pkt.Addressee))
	if addr == "" {
		return false
	}
	if snap.callSign != "" && strings.EqualFold(addr, snap.callSign) {
		return true
	}
	return c.heard != nil && c.heard.Heard(addr)
}

// passesFilter applies the effective filter for an igate-mode client. A
// listener-level filter (configured on the port) takes precedence over the
// client's own filter, matching the previous behaviour. It reads only the
// snapshot, the immutable server reference and the synchronised listener set.
func (c *TCPAPRSClient) passesFilter(snap deliverState, pkt parser.Parsed) bool {
	ctx := newFilterContext(snap.callSign)

	if c.server != nil {
		if l := listenerAt(c.server.index); l != nil {
			if lf := l.compiledFilter; lf != nil {
				return lf.Match(&pkt, ctx)
			}
		}
	}
	if snap.compiledFilter != nil {
		return snap.compiledFilter.Match(&pkt, ctx)
	}
	return false
}

// setFilter updates the client's filter string and recompiles it. An empty
// string clears the filter. Safe to call without holding c.mu (it does not
// touch the connection).
func (c *TCPAPRSClient) setFilter(f string) {
	c.filter = f
	if f == "" {
		c.compiledFilter = nil
		return
	}
	c.compiledFilter = filter.Compile(f)
}

// TCPAPRSServer provides a struct for APRS server
type TCPAPRSServer struct {
	// Server connection and management
	listener net.Listener
	clients  map[*TCPAPRSClient]bool
	mu       sync.RWMutex
	stopChan chan struct{}
	wg       sync.WaitGroup

	// Server configuration
	mode      client.Mode
	index     int
	tlsConfig *tls.Config                             // non-nil to serve TLS
	listenFn  func(addr string) (net.Listener, error) // listener factory (TCP by default)

	// Statistics (atomic counters)
	stats *model.Counters
}

// NewTCPAPRSServer creates a new APRS server
func NewTCPAPRSServer(mode client.Mode, index int) *TCPAPRSServer {
	return &TCPAPRSServer{
		clients:  make(map[*TCPAPRSClient]bool),
		stopChan: make(chan struct{}),
		mode:     mode,
		stats:    new(model.Counters),
		index:    index,
		listenFn: func(addr string) (net.Listener, error) { return upgrade.ListenTCP(addr) },
	}
}

// SetSCTP switches the server to listen on SCTP instead of TCP. Returns an
// error on platforms without SCTP support. Must be called before Start.
func (s *TCPAPRSServer) SetSCTP() error {
	if !sctpSupported() {
		return errSCTPUnsupported
	}
	s.listenFn = listenSCTP
	return nil
}

// SetTLS configures the server to serve TLS using the given certificate and
// key PEM files. If clientCA is non-empty, client certificates issued by that
// CA are requested and verified, enabling certificate-based login (a client
// may still authenticate by passcode if it presents no certificate). It must
// be called before Start.
func (s *TCPAPRSServer) SetTLS(certFile, keyFile, clientCA string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	if clientCA != "" {
		pem, err := os.ReadFile(clientCA)
		if err != nil {
			return err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return fmt.Errorf("no certificates found in client CA file %q", clientCA)
		}
		cfg.ClientCAs = pool
		// Verify a certificate when presented, but do not require one: clients
		// without a certificate fall back to passcode authentication.
		cfg.ClientAuth = tls.VerifyClientCertIfGiven
	}
	s.tlsConfig = cfg
	return nil
}

// Start an APRS server
func (s *TCPAPRSServer) Start(addr string) error {
	var err error
	s.listener, err = s.listenFn(addr)
	if err != nil {
		return err
	}

	// Wrap in TLS if configured.
	if s.tlsConfig != nil {
		s.listener = tls.NewListener(s.listener, s.tlsConfig)
		logger.L.Info(fmt.Sprintf("APRS listening on %s (TLS)", addr))
	} else {
		logger.L.Info(fmt.Sprintf("APRS listening on %s", addr))
	}

	// Start statistics updater for this server
	go s.updateStats()

	// Main server goroutine
	s.wg.Add(1)
	go s.handleServer()

	return nil
}

// Stop an APRS server and wait for its goroutines to drain.
func (s *TCPAPRSServer) Stop() {
	close(s.stopChan)
	if s.listener != nil {
		_ = s.listener.Close()
	}

	// Close all client connections
	s.mu.Lock()
	for c := range s.clients {
		c.Close()
	}
	s.mu.Unlock()

	// Wait for the accept loop and all client handlers to finish.
	s.wg.Wait()
}

// handleServer handles the main server loop for accepting connections
func (s *TCPAPRSServer) handleServer() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return
			default:
				logger.L.Error("Error accepting incoming client connection", zap.Error(err))
				continue
			}
		}

		s.wg.Add(1)
		go s.handleClient(conn)
	}
}

// globalClients is the process-wide count of connected TCP clients, used to
// enforce the global max_clients cap.
var globalClients atomic.Int64

// register adds a client to the server and updates online/peak counts. It
// returns false if a connection limit (per-listener or global) is exceeded, in
// which case the caller must reject the connection.
func (s *TCPAPRSServer) register(c *TCPAPRSClient) bool {
	// Check the global cap first (lock-free).
	if gm := config.Get().Server.MaxClients; gm > 0 {
		if globalClients.Load() >= int64(gm) {
			return false
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Per-listener cap.
	if l := listenerAt(s.index); l != nil && l.maxClients > 0 {
		if len(s.clients) >= l.maxClients {
			return false
		}
	}

	s.clients[c] = true
	globalClients.Add(1)
	if l := listenerAt(s.index); l != nil {
		l.setOnlineClient(len(s.clients))
	}
	return true
}

// unregister removes a client from the server.
func (s *TCPAPRSServer) unregister(c *TCPAPRSClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.clients[c]; ok {
		delete(s.clients, c)
		globalClients.Add(-1)
	}
	if l := listenerAt(s.index); l != nil {
		l.setOnlineClient(len(s.clients))
	}
}

// handleClient handles individual client connection
func (s *TCPAPRSServer) handleClient(conn net.Conn) {
	defer s.wg.Done()

	remoteAddr := conn.RemoteAddr().String()

	// Resolve this listener's settings (ACL, buffers, dupefeed) once.
	var (
		ibuf     = config.Get().Server.BuffSize * 1024
		obuf     = outQCap
		dupefeed = false
	)
	if l := listenerAt(s.index); l != nil {
		// Access-control: reject connections not permitted by the ACL.
		if !l.acl.Allow(remoteAddr) {
			logger.L.Info("Connection rejected by ACL", zap.String("remoteAddr", remoteAddr))
			_ = conn.Close()
			return
		}
		if l.ibufBytes > 0 {
			ibuf = l.ibufBytes
		}
		if l.obufBytes > 0 {
			// Convert byte budget to a line-count capacity (approx).
			obuf = l.obufBytes / 256
			if obuf < 16 {
				obuf = 16
			}
		}
		dupefeed = l.dupefeed
	}
	if ibuf <= 0 {
		ibuf = 1024
	}

	c := &TCPAPRSClient{
		conn:       conn,
		uptime:     time.Now(),
		lastActive: time.Now(),
		mode:       s.mode,
		dupefeed:   dupefeed,

		server:   s,
		dup:      historydb.NewDupeChecker(30 * time.Second),
		heard:    historydb.NewHeardListTTL(heardRetention),
		courtesy: historydb.NewHeardListTTL(courtesyRetention),
		sendCh:   make(chan []byte, obuf),

		stats: new(model.Counters),
	}

	// Enable TCP keepalive so dead peers are detected even when idle.
	applyKeepAlive(conn)

	// Enforce connection limits; reject politely when exceeded.
	if !s.register(c) {
		logger.L.Info("Connection rejected: client limit reached", zap.String("remoteAddr", remoteAddr))
		_, _ = conn.Write([]byte("# server full\r\n"))
		_ = conn.Close()
		return
	}
	logger.L.Info("Client connected", zap.String("remoteAddr", remoteAddr))

	defer func() {
		s.unregister(c)
		// Close() stops the heartbeat, the writer goroutine and the connection.
		c.Close()

		logger.L.Info("Client disconnected",
			zap.String("remoteAddr", remoteAddr),
			zap.String("callsign", c.callSign))
	}()

	// Start the async output writer and heartbeat.
	c.startWriter()
	c.startHeartbeat()

	// Send welcome message
	_ = c.Send(fmt.Sprintf("# %s %s/%s", meta.ENName, meta.Version, meta.Nickname))

	// Subscribe to data stream for this client
	c.dataCh, c.unsubscribe = uplink.Stream.Subscribe()
	go c.handleUplinkData()

	loginTimeout := loginTimeoutDur()
	clientTimeout := clientTimeoutDur()
	lineCount := 0
	reader := bufio.NewReaderSize(conn, ibuf)
	for {
		lineCount++

		// Bound the read by the login timeout until the client has logged in,
		// then by the (much longer) client idle timeout. Without this, a long
		// client_timeout would let a connection that never logs in block until
		// that timeout instead of being dropped promptly.
		readDeadline := clientTimeout
		if !c.loggedIn {
			readDeadline = loginTimeout
		}
		_ = conn.SetReadDeadline(time.Now().Add(readDeadline))

		// Disconnect clients that never log in.
		if time.Since(c.uptime) > loginTimeout && !c.loggedIn {
			return
		}

		// Read data from client
		line, err := reader.ReadString('\n')
		if err != nil {
			var netErr net.Error
			switch {
			case errors.As(err, &netErr) && netErr.Timeout():
				// A read timed out. Drop the link if it never logged in (past
				// the login window) or has been idle past the client timeout.
				if !c.loggedIn {
					return
				}
				if time.Since(c.lastActiveTime()) >= clientTimeout {
					return
				}
				continue
			case errors.Is(err, io.EOF):
				return
			}
			logger.L.Debug(fmt.Sprintf("Read error from %s", remoteAddr), zap.Error(err))
			return
		}

		// First line: detect and reject non-APRS probes (HTTP, TLS, junk) so
		// they are not mistaken for new APRS clients. APRS clients always send
		// "user ..." first.
		if lineCount == 1 && isNonAPRSProbe(line) {
			logger.L.Debug("Rejecting non-APRS probe connection",
				zap.String("remoteAddr", remoteAddr))
			return
		}

		// Trim whitespace
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Update last active time
		c.mu.Lock()
		c.lastActive = time.Now()
		c.mu.Unlock()

		// Update statistics for received bytes
		packetSize := uint64(len(line))
		c.stats.AddReceivedBytes(packetSize)
		s.stats.AddReceivedBytes(packetSize)
		globalStats.AddReceivedBytes(packetSize)

		// Process received packet
		s.processPacket(c, line)
	}
}

// lastActiveTime returns the client's last-active time under lock.
func (c *TCPAPRSClient) lastActiveTime() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastActive
}

// isNonAPRSProbe reports whether the first line from a connection looks like a
// non-APRS protocol probe (a web browser, a TLS client, or other junk) rather
// than an APRS-IS login. Such connections are common from scanners and from
// users mistakenly pointing an HTTP/HTTPS client at the APRS port; rejecting
// them prevents them from being counted as APRS clients.
func isNonAPRSProbe(raw string) bool {
	if raw == "" {
		return false
	}
	// TLS ClientHello starts with 0x16 (handshake) 0x03 (SSL/TLS major).
	if raw[0] == 0x16 {
		return true
	}
	// Reject control/binary leading bytes that valid APRS-IS lines never use.
	if raw[0] < 0x20 && raw[0] != '\r' && raw[0] != '\n' && raw[0] != '\t' {
		return true
	}
	// HTTP request lines.
	for _, m := range []string{"GET ", "POST ", "HEAD ", "PUT ", "DELETE ", "OPTIONS ", "CONNECT ", "TRACE "} {
		if strings.HasPrefix(raw, m) {
			return true
		}
	}
	return strings.Contains(raw, "HTTP/")
}

// tcpConnOf returns the underlying *net.TCPConn for a connection, unwrapping a
// TLS wrapper if present. It returns nil for non-TCP transports (e.g. SCTP).
func tcpConnOf(conn net.Conn) *net.TCPConn {
	if tc, ok := conn.(*net.TCPConn); ok {
		return tc
	}
	if tlsConn, ok := conn.(*tls.Conn); ok {
		if tc, ok := tlsConn.NetConn().(*net.TCPConn); ok {
			return tc
		}
	}
	return nil
}

// applyKeepAlive enables TCP keepalive with the configured idle/interval/count
// so an idle but dead peer is detected and dropped by the OS. It is a no-op on
// transports that are not TCP.
func applyKeepAlive(conn net.Conn) {
	tc := tcpConnOf(conn)
	if tc == nil {
		return
	}
	_ = tc.SetKeepAliveConfig(net.KeepAliveConfig{
		Enable:   true,
		Idle:     keepAliveIdle,
		Interval: keepAliveInterval,
		Count:    keepAliveCount,
	})
}

// setNoDelay disables Nagle's algorithm so packets are written promptly; high
// queuing delay can otherwise push duplicates past the dedup window. No-op on
// non-TCP transports.
func setNoDelay(conn net.Conn) {
	if tc := tcpConnOf(conn); tc != nil {
		_ = tc.SetNoDelay(true)
	}
}

// ClientCount returns count of active clients
func (s *TCPAPRSServer) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// kickOld disconnects old clients with same callsign.
//
// callSign is written under each client's own c.mu, so it must be read the same
// way. To avoid nesting c.mu inside s.mu, we first snapshot the candidate client
// pointers under s.mu, then release it and read each candidate's callSign under
// its own c.mu to decide whether to kick it.
func (s *TCPAPRSServer) kickOld(client *TCPAPRSClient, callsign string) {
	s.mu.RLock()
	candidates := make([]*TCPAPRSClient, 0, len(s.clients))
	for c := range s.clients {
		if client != c {
			candidates = append(candidates, c)
		}
	}
	s.mu.RUnlock()

	for _, c := range candidates {
		c.mu.Lock()
		match := c.callSign == callsign
		c.mu.Unlock()
		if !match {
			continue
		}
		logger.L.Info("Kicking old client with same callsign", zap.String("callsign", callsign))
		c.Close()
	}
}

// processPacket processes received packet from client
func (s *TCPAPRSServer) processPacket(c *TCPAPRSClient, packet string) {
	switch {
	case strings.HasPrefix(packet, "user "):
		s.handleLogin(c, packet)
	case strings.HasPrefix(packet, "#"):
		s.handleComment(c, packet)
	case strings.Contains(packet, ">"):
		s.handleAPRSData(c, packet)

		// Update statistics for received packets
		c.stats.AddReceivedPackets(1)
		s.stats.AddReceivedPackets(1)
		globalStats.AddReceivedPackets(1)
	default:
		c.stats.AddReceivedErrors(1)
		_ = c.Send("# invalid packet")
	}
}

// handleLogin processes user login command
func (s *TCPAPRSServer) handleLogin(client *TCPAPRSClient, packet string) {
	parts := strings.Fields(packet)
	if len(parts) < 4 || parts[0] != "user" {
		_ = client.Send("# invalid login")
		return
	}

	// Extract callsign and parameters (locals only; published under c.mu below).
	callSign := parts[1]
	passcode := ""
	software := ""
	version := ""
	filterSpec := ""
	for k, v := range parts {
		switch v {
		case "pass":
			if k+1 < len(parts) {
				passcode = parts[k+1]
			}
		case "vers":
			if k+2 < len(parts) {
				software = parts[k+1]
				version = parts[k+2]
			}
		// Server commands
		case serverCommandFilter:
			filterSpec = ""
			for i := 1; i < len(parts)-k; i++ {
				if parts[k+i] == serverCommandFilter {
					break
				}
				filterSpec += fmt.Sprintf("%s ", parts[k+i])
			}
			filterSpec = strings.TrimSuffix(filterSpec, " ")
		}
	}

	// Reject blacklisted login callsigns.
	if !security.LoginAllowed(callSign) {
		_ = client.Send("# login not allowed")
		logger.L.Warn("Login rejected by policy", zap.String("callsign", callSign))
		client.Close()
		return
	}

	// Publish the client's identity, version and filter, and validate the
	// login, all under c.mu so the status updater and kickOld observe a
	// consistent view (these fields are read while holding c.mu elsewhere).
	// A login is accepted either by a matching passcode or, on a TLS
	// connection, by a client certificate whose callsign matches.
	intPasscode, _ := strconv.Atoi(passcode)
	client.mu.Lock()
	client.callSign = callSign
	client.software = software
	client.version = version
	client.setFilter(filterSpec) // compiles the login-supplied filter (igate)

	byPass := aprsutils.Passcode(callSign) == intPasscode
	byCert := !byPass && verifiedByCert(client.conn, callSign)
	if byPass || byCert {
		client.verified = true
		setNoDelay(client.conn)
		_ = client.Send(fmt.Sprintf("# logresp %s verified, server %s", callSign, config.Get().Server.ID))
		logger.L.Info("Client logged in",
			zap.String("callsign", callSign), zap.Bool("verified", true),
			zap.Bool("cert", byCert))
	} else {
		_ = client.Send(fmt.Sprintf("# logresp %s unverified, server %s", callSign, config.Get().Server.ID))
		logger.L.Warn("Client login unverified - invalid passcode", zap.String("callsign", callSign))
	}
	client.loggedIn = true
	client.mu.Unlock()

	// Disconnect old clients with the same callsign (after the callsign is
	// published so kickOld sees it and skips this connection).
	s.kickOld(client, callSign)
}

// handleComment processes comment/keepalive lines and in-band server commands.
// A "#filter <spec>" line replaces (not appends) the client's filter.
func (s *TCPAPRSServer) handleComment(client *TCPAPRSClient, packet string) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(packet, "#"))

	if strings.HasPrefix(trimmed, serverCommandFilter) {
		spec := strings.TrimSpace(strings.TrimPrefix(trimmed, serverCommandFilter))
		client.mu.Lock()
		client.setFilter(spec)
		client.mu.Unlock()
		logger.L.Debug("Client updated filter",
			zap.String("callsign", client.callSign), zap.String("filter", spec))
		return
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	_ = client.Send("# pong")
}

// handleAPRSData processes APRS data packets
func (s *TCPAPRSServer) handleAPRSData(c *TCPAPRSClient, packet string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Dupefeed ports are receive-only: clients there never inject traffic.
	if c.dupefeed {
		return
	}

	// Unverified clients may not relay traffic when the policy forbids it.
	if !c.verified {
		if security.DisallowUnverified() {
			c.stats.AddReceivedErrors(1)
			return
		}
		_ = c.Send("# invalid login")
		return
	}

	// Parse APRS packet (lenient on the destination call).
	parsed, _ := parser.Parse(packet, parser.WithDisableToCallsignValidate())
	if parsed.To == "" {
		c.stats.AddReceivedErrors(1)
		return
	}

	// Reject packets from blacklisted / bogus source callsigns.
	if !security.SourceAllowed(parsed.From) {
		c.stats.AddReceivedErrors(1)
		return
	}

	// Drop duplicates seen within the dedup window, but forward them to any
	// dupefeed ports for debugging.
	if c.dup.Seen(packet) {
		c.stats.AddReceivedDups(1)
		globalStats.AddReceivedDups(1)
		uplink.Stream.WriteDupe(parsed, c.callSign)
		return
	}

	// Process QConstruct for packet routing.
	qConfig := &qConstruct.QConfig{
		ServerLogin:            config.Get().Server.ID,
		ClientLogin:            c.callSign,
		ConnectionType:         qConstruct.ConnectionVerified,
		IsVerified:             true,
		QProtocolID:            security.QProtocolID(),
		DisallowOtherProtocols: security.DisallowOtherQProtocols(),
	}
	result, err := qConstruct.QConstruct(parsed, qConfig)
	if err != nil || result.ShouldDrop || result.IsLoop {
		c.stats.AddReceivedQDrop(1)
		return
	}

	// Replace path in packet
	packet, err = qConstruct.Replace(packet, parsed.To, result.Path)
	if err != nil {
		c.stats.AddReceivedErrors(1)
		return
	}

	// Reparse modified packet
	parsed, err = parser.Parse(packet, parser.WithDisableToCallsignValidate())
	if err != nil {
		c.stats.AddReceivedErrors(1)
		return
	}

	// Record the source station as heard by this client (message routing).
	if parsed.From != "" && c.heard != nil {
		c.heard.Add(parsed.From)
	}

	// Send to distribution stream
	uplink.Stream.Write(parsed, c.callSign)
}

// updateServerSendStats updates server and global send statistics.
func (s *TCPAPRSServer) updateServerSendStats(packets uint64, bytes uint64) {
	s.stats.AddSentPackets(packets)
	s.stats.AddSentBytes(bytes)
	globalStats.AddSentPackets(packets)
	globalStats.AddSentBytes(bytes)
}

// updateStats updates server statistics rates every second
func (s *TCPAPRSServer) updateStats() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.stats.UpdateRates()
			if l := listenerAt(s.index); l != nil {
				l.SetStats(s.stats.Snapshot())
			}

			// Update per-client rates and publish global rates.
			s.updateClientRates()
			globalStats.UpdateRates()
		case <-s.stopChan:
			return
		}
	}
}

// updateClientRates updates rates for all connected clients
func (s *TCPAPRSServer) updateClientRates() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for c := range s.clients {
		c.stats.UpdateRates()
	}
}
