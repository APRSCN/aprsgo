package listener

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/internal/uplink"
	"github.com/APRSCN/aprsutils"
	"github.com/APRSCN/aprsutils/client"
	"github.com/APRSCN/aprsutils/parser"
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
	server      *TCPAPRSServer

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
	currentSentPackets := atomic.LoadUint64(&globalStats.SentPackets)
	currentReceivedPackets := atomic.LoadUint64(&globalStats.ReceivedPackets)
	currentSentBytes := atomic.LoadUint64(&globalStats.SentBytes)
	currentReceivedBytes := atomic.LoadUint64(&globalStats.ReceivedBytes)

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

		Clients[c].Stats = *c.stats

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
		stats:      new(model.Statistics),
		server:     s,
	}

	// Register a client
	s.mu.Lock()
	s.clients[c] = true
	Clients[c] = &Client{
		At:     s.listener.Addr().String(),
		Addr:   c.conn.RemoteAddr().String(),
		Uptime: time.Now(),
		Last:   c.lastActive,
		c:      c,
		Stats:  *c.stats,
	}
	Listeners[s.index].OnlineClient = len(s.clients)
	if Listeners[s.index].OnlineClient > Listeners[s.index].PeakClient {
		Listeners[s.index].PeakClient = Listeners[s.index].OnlineClient
	}
	s.mu.Unlock()

	defer func() {
		// Delete a client
		s.mu.Lock()
		delete(s.clients, c)
		delete(Clients, c)
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
		Clients[c].Last = c.lastActive

		// Update statistics for received data
		packetSize := uint64(len(line))
		atomic.AddUint64(&c.stats.ReceivedBytes, packetSize)
		atomic.AddUint64(&s.stats.ReceivedBytes, packetSize)
		atomic.AddUint64(&globalStats.ReceivedBytes, packetSize)

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
		atomic.AddUint64(&c.stats.ReceivedPackets, 1)
		atomic.AddUint64(&s.stats.ReceivedPackets, 1)
		atomic.AddUint64(&globalStats.ReceivedPackets, 1)
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
	Clients[client].ID = callSign
	Clients[client].Software = client.software
	Clients[client].Version = client.version
	Clients[client].Filter = client.filter

	// Kick old client
	for k, v := range Clients {
		if k != client && v.ID == callSign {
			v.c.Close()
		}
	}

	// Calc passcode
	intPasscode, _ := strconv.Atoi(passcode)

	// Check passcode
	if aprsutils.Passcode(callSign) == intPasscode {
		client.mu.Lock()
		client.callSign = callSign
		client.verified = true
		Clients[client].Verified = true
		client.mu.Unlock()

		_ = client.Send(fmt.Sprintf("# logresp %s verified, server %s", callSign, config.C.GetString("server.id")))
		logger.L.Debug(fmt.Sprintf("OnlineClient logged in as %s", callSign), zap.String("client", client.conn.RemoteAddr().String()))
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
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.verified {
		_ = c.Send("# invalid login")
		return
	}

	uplink.Stream.Write(packet, Clients[c].ID)
}

// handleUplinkData sends data to client
func (c *TCPAPRSClient) handleUplinkData() {
	for data := range c.dataCh {
		c.mu.Lock()
		if c.loggedIn && c.conn != nil && Clients[c] != nil && data.Writer != Clients[c].ID {
			switch c.mode {
			case client.Fullfeed:
				_ = c.Send(data.Data)
				atomic.AddUint64(&c.stats.SentPackets, 1)
			case client.IGate:
				if len(Listeners) > c.server.index && Listeners[c.server.index].Filter != "" {
					// Parse APRS packet
					parsed, err := parser.Parse(data.Data)
					if err == nil {
						if Filter(Listeners[c.server.index].Filter, parsed) {
							_ = c.Send(data.Data)
							atomic.AddUint64(&c.stats.SentPackets, 1)
						}
					}
				} else {
					if c.filter != "" {
						// Parse APRS packet
						parsed, err := parser.Parse(data.Data)
						if err == nil {
							if Filter(c.filter, parsed) {
								_ = c.Send(data.Data)
								atomic.AddUint64(&c.stats.SentPackets, 1)
							}
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
		atomic.AddUint64(&c.stats.SentBytes, packetSize)
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
					delete(Clients, c)
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
	atomic.AddUint64(&s.stats.SentPackets, packets)
	atomic.AddUint64(&s.stats.SentBytes, bytes)
	atomic.AddUint64(&globalStats.SentPackets, packets)
	atomic.AddUint64(&globalStats.SentBytes, bytes)
}
