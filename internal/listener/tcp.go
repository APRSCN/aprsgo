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

// TCPAPRSClient provides a struct for APRS client connection
type TCPAPRSClient struct {
	conn        net.Conn
	callSign    string
	verified    bool
	loggedIn    bool
	lastActive  time.Time
	software    string
	version     string
	mu          sync.Mutex
	unsubscribe func()
	dataCh      <-chan uplink.StreamData
	mode        client.Mode
	filter      string

	server *TCPAPRSServer
	dup    *historydb.MapFloat64History

	stats *model.Statistics // Client statistics
}

// TCPAPRSServer provides a struct for APRS server
type TCPAPRSServer struct {
	port     int
	listener net.Listener
	clients  map[*TCPAPRSClient]bool
	mu       sync.RWMutex
	stopChan chan struct{}
	mode     client.Mode
	index    int

	stats *model.Statistics // Server statistics
}

// Global statistics for all servers
var (
	globalStats model.Statistics
	statsMutex  sync.RWMutex
)

// updateAllRates updates rates for global Stats and all clients
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

	// Connection clean goroutine
	go s.cleanupInactiveClients()

	// Main server goroutine
	go s.handleServer()

	return nil
}

// handleServer handles the main server
func (s *TCPAPRSServer) handleServer() {
	// Accept connection
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

		GetWithoutOK(c).Stats = *c.stats

		c.mu.Unlock()
	}
}

// Stop an APRS server
func (s *TCPAPRSServer) Stop() {
	close(s.stopChan)
	if s.listener != nil {
		_ = s.listener.Close()
	}

	// Close all client connection
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		c.Close()
	}
}

// handleClient handles connection from client
func (s *TCPAPRSServer) handleClient(conn net.Conn) {
	c := &TCPAPRSClient{
		conn:       conn,
		lastActive: time.Now(),
		mode:       s.mode,

		server: s,
		dup:    historydb.NewMapFloat64History(),

		stats: new(model.Statistics),
	}

	// Register a client
	s.mu.Lock()
	s.clients[c] = true
	Set(c, &Client{
		At:     s.listener.Addr().String(),
		Addr:   c.conn.RemoteAddr().String(),
		Uptime: time.Now(),
		Last:   c.lastActive,
		c:      c,
		Stats:  *c.stats,
	})
	Listeners[s.index].OnlineClient = len(s.clients)
	if Listeners[s.index].OnlineClient > Listeners[s.index].PeakClient {
		Listeners[s.index].PeakClient = Listeners[s.index].OnlineClient
	}
	s.mu.Unlock()

	defer func() {
		// Delete a client
		s.mu.Lock()
		delete(s.clients, c)
		Del(c)
		if len(Listeners) > s.index {
			Listeners[s.index].OnlineClient = len(s.clients)
		}
		s.mu.Unlock()
		c.Close()
	}()

	logger.L.Debug("New client connected", zap.String("client", c.conn.RemoteAddr().String()))

	// Send welcome
	_ = c.Send(fmt.Sprintf("# %s %s/%s", config.ENName, config.Version, config.Nickname))

	// Subscribe data for this client
	c.dataCh, c.unsubscribe = uplink.Stream.Subscribe()
	go c.handleUplinkData()

	reader := bufio.NewReader(conn)
	for {
		// Set timeout
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))

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

		// Trim space
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Record last activate status
		c.lastActive = time.Now()
		GetWithoutOK(c).Last = c.lastActive

		// Update statistics for received data
		packetSize := uint64(len(line))
		c.stats.ReceivedBytes += packetSize
		s.stats.ReceivedBytes += packetSize
		globalStats.ReceivedBytes += packetSize

		// Process received data
		s.processPacket(c, line)
	}
}

// processPacket processes received data
func (s *TCPAPRSServer) processPacket(c *TCPAPRSClient, packet string) {
	// Parse packet
	if strings.HasPrefix(packet, "user ") {
		s.handleLogin(c, packet)
	} else if strings.HasPrefix(packet, "#") {
		s.handleComment(c)
	} else if strings.Contains(packet, ">") {
		s.handleAPRSData(c, packet)

		// Update statistics for received data
		c.stats.ReceivedPackets += 1
		s.stats.ReceivedPackets += 1
		globalStats.ReceivedPackets += 1
	} else {
		_ = c.Send("# invalid packet")
	}
}

// handleLogin handles login command
func (s *TCPAPRSServer) handleLogin(client *TCPAPRSClient, packet string) {
	// Parse
	parts := strings.Fields(packet)
	if len(parts) < 4 {
		_ = client.Send("# invalid login")
		return
	}

	if parts[0] != "user" {
		_ = client.Send("# invalid login")
		return
	}

	// Get callsign and passcode
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
	// Record client
	GetWithoutOK(client).ID = callSign
	client.callSign = callSign
	GetWithoutOK(client).Software = client.software
	GetWithoutOK(client).Version = client.version
	GetWithoutOK(client).Filter = client.filter

	// Kick old client
	KickOld(client, callSign)

	// Calc passcode
	intPasscode, _ := strconv.Atoi(passcode)

	// Check passcode
	if aprsutils.Passcode(callSign) == intPasscode {
		client.mu.Lock()

		client.verified = true
		GetWithoutOK(client).Verified = true

		_ = client.Send(fmt.Sprintf("# logresp %s verified, server %s", callSign, config.C.GetString("server.id")))
		logger.L.Debug(fmt.Sprintf("OnlineClient logged in as %s", callSign), zap.String("client", client.conn.RemoteAddr().String()))

		client.mu.Unlock()
	} else {
		_ = client.Send(fmt.Sprintf("# logresp %s unverified, server %s", callSign, config.C.GetString("server.id")))
	}
	client.loggedIn = true
}

// handleComment handles
func (s *TCPAPRSServer) handleComment(client *TCPAPRSClient) {
	client.mu.Lock()
	defer client.mu.Unlock()

	_ = client.Send("# pong")
}

// handleAPRSData handles APRS packet
func (s *TCPAPRSServer) handleAPRSData(c *TCPAPRSClient, packet string) {
	// Get time now
	now := time.Now()

	// Lock
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.verified {
		_ = c.Send("# invalid login")
		return
	}

	// Hash the data (dup check)
	h64 := fnv.New64a()
	_, err := h64.Write([]byte(packet))
	if err == nil {
		hash64 := h64.Sum64()

		// Clear first
		c.dup.ClearByValue(30)

		if c.dup.Contain(hash64) {
			// Drop dup
			c.stats.ReceivedDups++
			return
		}

		go c.dup.Record(hash64, float64(now.UnixNano())/1e9)
	}

	// Try to parse
	parsed, err := parser.Parse(packet)
	if err != nil {
		// Drop error
		c.stats.ReceivedErrors++
		return
	}

	// Process QConstruct
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

	// Replace path
	packet = qConstruct.Replace(packet, parsed.Path, result.Path)
	parsed, err = parser.Parse(packet)
	if err != nil {
		// Drop error
		c.stats.ReceivedErrors++
		return
	}

	uplink.Stream.Write(parsed, c.callSign)
}

// handleUplinkData sends data to client
func (c *TCPAPRSClient) handleUplinkData() {
	for data := range c.dataCh {
		c.mu.Lock()
		if c.loggedIn && c.conn != nil && data.Writer != c.callSign {
			switch c.mode {
			case client.Fullfeed:
				_ = c.Send(data.Data.Raw)
				c.stats.SentPackets += 1
			case client.IGate:
				if len(Listeners) > c.server.index && Listeners[c.server.index].Filter != "" {
					if Filter(Listeners[c.server.index].Filter, data.Data) {
						_ = c.Send(data.Data.Raw)
						c.stats.SentPackets += 1
					}
				} else {
					if c.filter != "" {
						if Filter(c.filter, data.Data) {
							_ = c.Send(data.Data.Raw)
							c.stats.SentPackets += 1
						}
					}
				}
			}
		}
		c.mu.Unlock()
		//time.Sleep(time.Millisecond * 10)
	}
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

	if c.unsubscribe != nil {
		c.unsubscribe()
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

// cleanupInactiveClients cleans inactivate client
func (s *TCPAPRSServer) cleanupInactiveClients() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for c := range s.clients {
				c.mu.Lock()
				if now.Sub(c.lastActive) > 15*time.Minute {
					c.Close()
					delete(s.clients, c)
					Del(c)
					Listeners[s.index].OnlineClient = len(s.clients)
				}
				c.mu.Unlock()
			}
			s.mu.Unlock()
		case <-s.stopChan:
			return
		}
	}
}

// ClientCount return count of activate clients
func (s *TCPAPRSServer) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// UpdateServerSendStats updates server send statistics (called when sending data to clients)
func (s *TCPAPRSServer) UpdateServerSendStats(packets uint64, bytes uint64) {
	s.stats.SentPackets += packets
	s.stats.SentBytes += bytes
	globalStats.SentPackets += packets
	globalStats.SentBytes += bytes
}
