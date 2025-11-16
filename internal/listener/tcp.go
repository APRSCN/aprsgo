package listener

import (
	"bufio"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/historydb"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/internal/uplink"
	"github.com/APRSCN/aprsutils"
	"github.com/APRSCN/aprsutils/client"
	"github.com/APRSCN/aprsutils/parser"
	"github.com/APRSCN/aprsutils/qConstruct"
	"go.uber.org/zap"
)

var serverCommands = []string{
	"filter",
}

// Global statistics for all servers
var (
	globalStats model.Statistics
	statsMutex  sync.RWMutex
)

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

	// Server reference and duplicate checking
	server *TCPAPRSServer
	dup    *historydb.MapFloat64History

	// Statistics
	stats *model.Statistics

	// Heartbeat management
	heartbeatStopChan chan struct{}
	heartbeatMutex    sync.Mutex
}

// Send data to client
func (c *TCPAPRSClient) Send(data string) error {
	if c.conn == nil {
		return fmt.Errorf("connection closed")
	}

	_, err := fmt.Fprintf(c.conn, "%s\n", data)
	if err == nil {
		// Update send statistics
		packetSize := uint64(len(data))
		c.stats.SentBytes += packetSize
		c.server.UpdateServerSendStats(1, packetSize)
	}
	return err
}

// Close connection of client
func (c *TCPAPRSClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Stop heartbeat
	c.stopHeartbeat()

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
					c.sendHeartbeatWithRetry()
				}

			case <-c.heartbeatStopChan:
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

// sendHeartbeatWithRetry sends heartbeat with retry mechanism
func (c *TCPAPRSClient) sendHeartbeatWithRetry() {
	maxRetries := 3
	retryInterval := 2 * time.Second

	for retry := 0; retry < maxRetries; retry++ {
		c.mu.Lock()
		// Double-check connection is still valid after acquiring lock
		if c.conn == nil {
			c.mu.Unlock()
			return
		}

		err := c.Send(fmt.Sprintf(
			"# %s-%s %s %s %s",
			config.ENName, config.Nickname, config.Version,
			time.Now().Format(time.RFC1123),
			config.C.GetString("server.id"),
		))
		c.mu.Unlock()

		if err == nil {
			logger.L.Debug("Heartbeat sent successfully",
				zap.String("client", c.conn.RemoteAddr().String()),
				zap.String("callsign", c.callSign))
			return
		}

		logger.L.Debug("Heartbeat send failed, retrying",
			zap.String("client", c.conn.RemoteAddr().String()),
			zap.String("callsign", c.callSign),
			zap.Int("retry", retry+1),
			zap.Error(err))

		if retry < maxRetries-1 {
			time.Sleep(retryInterval)

			// Check if we should stop retrying
			c.mu.Lock()
			if c.conn == nil {
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()
		}
	}

	logger.L.Debug("Heartbeat failed after all retries, disconnecting client",
		zap.String("client", c.conn.RemoteAddr().String()),
		zap.String("callsign", c.callSign))

	c.Close()
}

// handleUplinkData sends data to client from uplink stream
func (c *TCPAPRSClient) handleUplinkData() {
	for data := range c.dataCh {
		c.mu.Lock()
		if c.loggedIn && c.conn != nil && data.Writer != c.callSign {
			switch c.mode {
			case client.Fullfeed:
				_ = c.Send(data.Data.Raw)
				c.stats.SentPackets++
			case client.IGate:
				if len(Listeners) > c.server.index && Listeners[c.server.index].Filter != "" {
					if Filter(Listeners[c.server.index].Filter, data.Data) {
						_ = c.Send(data.Data.Raw)
						c.stats.SentPackets++
					}
				} else {
					if c.filter != "" {
						if Filter(c.filter, data.Data) {
							_ = c.Send(data.Data.Raw)
							c.stats.SentPackets++
						}
					}
				}
			}
		}
		c.mu.Unlock()
	}
}

// TCPAPRSServer provides a struct for APRS server
type TCPAPRSServer struct {
	// Server connection and management
	port     int
	listener net.Listener
	clients  map[*TCPAPRSClient]bool
	mu       sync.RWMutex
	stopChan chan struct{}

	// Server configuration
	mode  client.Mode
	index int

	// Statistics
	stats *model.Statistics
}

// NewTCPAPRSServer creates a new APRS server
func NewTCPAPRSServer(mode client.Mode, index int) *TCPAPRSServer {
	return &TCPAPRSServer{
		clients:  make(map[*TCPAPRSClient]bool),
		stopChan: make(chan struct{}),
		mode:     mode,
		stats:    new(model.Statistics),
		index:    index,
	}
}

// Start an APRS server
func (s *TCPAPRSServer) Start(addr string) error {
	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	logger.L.Info(fmt.Sprintf("APRS listening on %s", addr))

	// Start statistics updater for this server
	go s.updateStats()

	// Main server goroutine
	go s.handleServer()

	return nil
}

// Stop an APRS server
func (s *TCPAPRSServer) Stop() {
	close(s.stopChan)
	if s.listener != nil {
		_ = s.listener.Close()
	}

	// Close all client connections
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		c.Close()
	}
}

// handleServer handles the main server loop for accepting connections
func (s *TCPAPRSServer) handleServer() {
	// Accept connections
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

		go s.handleClient(conn)
	}
}

// handleClient handles individual client connection
func (s *TCPAPRSServer) handleClient(conn net.Conn) {
	c := &TCPAPRSClient{
		conn:       conn,
		uptime:     time.Now(),
		lastActive: time.Now(),
		mode:       s.mode,

		server: s,
		dup:    historydb.NewMapFloat64History(),

		stats: new(model.Statistics),
	}

	// Register client
	s.mu.Lock()
	s.clients[c] = true
	if len(Listeners) > s.index {
		Listeners[s.index].OnlineClient = len(s.clients)
		if Listeners[s.index].OnlineClient > Listeners[s.index].PeakClient {
			Listeners[s.index].PeakClient = Listeners[s.index].OnlineClient
		}
	}
	s.mu.Unlock()

	logger.L.Info("Client connected",
		zap.String("remoteAddr", conn.RemoteAddr().String()),
		zap.Int("totalClients", len(s.clients)))

	defer func() {
		// Stop heartbeat
		c.stopHeartbeat()

		// Remove client
		s.mu.Lock()
		delete(s.clients, c)
		if len(Listeners) > s.index {
			Listeners[s.index].OnlineClient = len(s.clients)
		}
		s.mu.Unlock()

		// Close connection and unsubscribe
		if c.unsubscribe != nil {
			c.unsubscribe()
		}
		if c.conn != nil {
			_ = c.conn.Close()
			c.conn = nil
		}

		logger.L.Info("Client disconnected",
			zap.String("remoteAddr", conn.RemoteAddr().String()),
			zap.String("callsign", c.callSign),
			zap.Int("remainingClients", len(s.clients)))
	}()

	// Start heartbeat
	c.startHeartbeat()

	// Send welcome message
	_ = c.Send(fmt.Sprintf("# %s %s/%s", config.ENName, config.Version, config.Nickname))

	// Subscribe to data stream for this client
	c.dataCh, c.unsubscribe = uplink.Stream.Subscribe()
	go c.handleUplinkData()

	lineCount := 0
	reader := bufio.NewReaderSize(conn, config.C.GetInt("server.bufSize")*1024)
	for {
		// Add count
		lineCount++

		// Set read timeout
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// 30s not send login
		if time.Now().Sub(c.uptime) > 30*time.Second && !c.loggedIn {
			return
		}

		// Read data from client
		line, err := reader.ReadString('\n')
		if err != nil {
			var netErr net.Error
			switch {
			case errors.As(err, &netErr) && netErr.Timeout():
				continue
			case errors.Is(err, io.EOF):
				return
			}
			logger.L.Debug(fmt.Sprintf("Read error from %s", conn.RemoteAddr().String()), zap.Error(err))
			return
		}

		// Trim whitespace
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Why you're using a browser to visit the APRS port?
		if lineCount == 1 && strings.Contains(line, "HTTP/") {
			return
		}

		// Update last active time
		c.lastActive = time.Now()

		// Update statistics for received data
		packetSize := uint64(len(line))
		c.stats.ReceivedBytes += packetSize
		s.stats.ReceivedBytes += packetSize
		globalStats.ReceivedBytes += packetSize

		// Process received packet
		s.processPacket(c, line)
	}
}

// ClientCount returns count of active clients
func (s *TCPAPRSServer) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// kickOld disconnects old clients with same callsign
func (s *TCPAPRSServer) kickOld(client *TCPAPRSClient, callsign string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		if client != c && c.callSign == callsign {
			logger.L.Info("Kicking old client with same callsign",
				zap.String("callsign", callsign),
				zap.String("oldClient", c.conn.RemoteAddr().String()),
				zap.String("newClient", client.conn.RemoteAddr().String()))
			c.Close()
		}
	}
}

// processPacket processes received packet from client
func (s *TCPAPRSServer) processPacket(c *TCPAPRSClient, packet string) {
	// Parse packet type and route to appropriate handler
	if strings.HasPrefix(packet, "user ") {
		s.handleLogin(c, packet)
	} else if strings.HasPrefix(packet, "#") {
		s.handleComment(c)
	} else if strings.Contains(packet, ">") {
		s.handleAPRSData(c, packet)

		// Update statistics for received packets
		c.stats.ReceivedPackets++
		s.stats.ReceivedPackets++
		globalStats.ReceivedPackets++
	} else {
		c.stats.ReceivedErrors++
		_ = c.Send("# invalid packet")
	}
}

// handleLogin processes user login command
func (s *TCPAPRSServer) handleLogin(client *TCPAPRSClient, packet string) {
	// Parse login command
	parts := strings.Fields(packet)
	if len(parts) < 4 {
		_ = client.Send("# invalid login")
		return
	}

	if parts[0] != "user" {
		_ = client.Send("# invalid login")
		return
	}

	// Extract callsign and parameters
	callSign := parts[1]
	passcode := ""
	for k, v := range parts {
		switch v {
		case "pass":
			passcode = parts[k+1]
		case "vers":
			client.software = parts[k+1]
			client.version = parts[k+2]
		// Server commands
		case "filter":
			client.filter = ""
			for i := 1; i < len(parts)-k; i++ {
				if slices.Contains(serverCommands, parts[k+i]) {
					break
				}
				client.filter += fmt.Sprintf("%s ", parts[k+i])
			}
			client.filter = strings.TrimSuffix(client.filter, " ")
		}
	}

	// Record client callsign
	client.callSign = callSign

	// Disconnect old clients with same callsign
	s.kickOld(client, callSign)

	// Validate passcode
	intPasscode, _ := strconv.Atoi(passcode)
	if aprsutils.Passcode(callSign) == intPasscode {
		client.mu.Lock()
		client.verified = true
		_ = client.Send(fmt.Sprintf("# logresp %s verified, server %s", callSign, config.C.GetString("server.id")))
		logger.L.Info("Client logged in successfully",
			zap.String("callsign", callSign),
			zap.String("client", client.conn.RemoteAddr().String()))
		client.mu.Unlock()
	} else {
		_ = client.Send(fmt.Sprintf("# logresp %s unverified, server %s", callSign, config.C.GetString("server.id")))
		logger.L.Warn("Client login failed - invalid passcode",
			zap.String("callsign", callSign),
			zap.String("client", client.conn.RemoteAddr().String()))
	}
	client.loggedIn = true
}

// handleComment processes comment/heartbeat packets
func (s *TCPAPRSServer) handleComment(client *TCPAPRSClient) {
	client.mu.Lock()
	defer client.mu.Unlock()
	_ = client.Send("# pong")
}

// handleAPRSData processes APRS data packets
func (s *TCPAPRSServer) handleAPRSData(c *TCPAPRSClient, packet string) {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.verified {
		_ = c.Send("# invalid login")
		return
	}

	// Duplicate checking using hash
	h64 := fnv.New64a()
	_, err := h64.Write([]byte(packet))
	if err == nil {
		hash64 := h64.Sum64()

		// Clear old entries first
		c.dup.ClearByValue(30)

		if c.dup.Contain(hash64) {
			// Drop duplicate packet
			c.stats.ReceivedDups++
			return
		}

		go c.dup.Record(hash64, float64(now.UnixNano())/1e9)
	}

	// Parse APRS packet
	parsed, _ := parser.Parse(packet, parser.WithDisableToCallsignValidate())
	if parsed.To == "" {
		// Drop parsing errors
		c.stats.ReceivedErrors++
		return
	}

	// Process QConstruct for packet routing
	qConfig := &qConstruct.QConfig{
		ServerLogin:    config.C.GetString("server.id"),
		ClientLogin:    c.callSign,
		ConnectionType: qConstruct.ConnectionVerified,
		EnableTrace:    false,
		IsVerified:     true,
	}
	result, err := qConstruct.QConstruct(parsed, qConfig)
	if err != nil || result.ShouldDrop || result.IsLoop {
		c.stats.ReceivedQDrop++
		return
	}

	// Replace path in packet
	packet, err = qConstruct.Replace(packet, parsed.To, result.Path)
	if err != nil {
		c.stats.ReceivedErrors++
		return
	}

	// Reparse modified packet
	parsed, err = parser.Parse(packet)
	if err != nil {
		c.stats.ReceivedErrors++
		return
	}

	// Send to uplink stream
	uplink.Stream.Write(parsed, c.callSign)
}

// UpdateServerSendStats updates server send statistics
func (s *TCPAPRSServer) UpdateServerSendStats(packets uint64, bytes uint64) {
	s.stats.SentPackets += packets
	s.stats.SentBytes += bytes
	globalStats.SentPackets += packets
	globalStats.SentBytes += bytes
}

// updateStats updates server statistics rates every second
func (s *TCPAPRSServer) updateStats() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			// Update server rates
			currentSentPackets := s.stats.SentPackets
			currentReceivedPackets := s.stats.ReceivedPackets
			currentSentBytes := s.stats.SentBytes
			currentReceivedBytes := s.stats.ReceivedBytes

			s.stats.SendPacketRate = currentSentPackets - s.stats.LastSentPackets
			s.stats.RecvPacketRate = currentReceivedPackets - s.stats.LastReceivedPackets
			s.stats.SendByteRate = currentSentBytes - s.stats.LastSentBytes
			s.stats.RecvByteRate = currentReceivedBytes - s.stats.LastReceivedBytes

			s.stats.LastSentPackets = currentSentPackets
			s.stats.LastReceivedPackets = currentReceivedPackets
			s.stats.LastSentBytes = currentSentBytes
			s.stats.LastReceivedBytes = currentReceivedBytes

			Listeners[s.index].Stats = *s.stats

			s.mu.Unlock()

			// Update client rates
			s.updateClientRates()
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
		c.mu.Lock()
		currentSentPackets := c.stats.SentPackets
		currentReceivedPackets := c.stats.ReceivedPackets
		currentSentBytes := c.stats.SentBytes
		currentReceivedBytes := c.stats.ReceivedBytes

		c.stats.SendPacketRate = currentSentPackets - c.stats.LastSentPackets
		c.stats.RecvPacketRate = currentReceivedPackets - c.stats.LastReceivedPackets
		c.stats.SendByteRate = currentSentBytes - c.stats.LastSentBytes
		c.stats.RecvByteRate = currentReceivedBytes - c.stats.LastReceivedBytes

		c.stats.LastSentPackets = currentSentPackets
		c.stats.LastReceivedPackets = currentReceivedPackets
		c.stats.LastSentBytes = currentSentBytes
		c.stats.LastReceivedBytes = currentReceivedBytes

		c.mu.Unlock()
	}
}

// updateAllRates updates rates for global stats and all clients
func updateAllRates() {
	statsMutex.Lock()
	defer statsMutex.Unlock()

	// Update global rates
	currentSentPackets := globalStats.SentPackets
	currentReceivedPackets := globalStats.ReceivedPackets
	currentSentBytes := globalStats.SentBytes
	currentReceivedBytes := globalStats.ReceivedBytes

	globalStats.SendPacketRate = currentSentPackets - globalStats.LastSentPackets
	globalStats.RecvPacketRate = currentReceivedPackets - globalStats.LastReceivedPackets
	globalStats.SendByteRate = currentSentBytes - globalStats.LastSentBytes
	globalStats.RecvByteRate = currentReceivedBytes - globalStats.LastReceivedBytes

	globalStats.LastSentPackets = currentSentPackets
	globalStats.LastReceivedPackets = currentReceivedPackets
	globalStats.LastSentBytes = currentSentBytes
	globalStats.LastReceivedBytes = currentReceivedBytes
}
