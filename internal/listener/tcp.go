package listener

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/uplink"
	"github.com/APRSCN/aprsutils"
	"github.com/APRSCN/aprsutils/client"
	"go.uber.org/zap"
)

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
}

// TCPAPRSServer provides a struct for APRS server
type TCPAPRSServer struct {
	port     int
	listener net.Listener
	clients  map[*TCPAPRSClient]bool
	mu       sync.RWMutex
	stopChan chan struct{}
	mode     client.Mode
}

// NewTCPAPRSServer creates a new APRS server
func NewTCPAPRSServer(mode client.Mode) *TCPAPRSServer {
	return &TCPAPRSServer{
		clients:  make(map[*TCPAPRSClient]bool),
		stopChan: make(chan struct{}),
		mode:     mode,
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

	// Connection clean goroutine
	go s.cleanupInactiveClients()

	// Accept connection
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return nil
			default:
				logger.L.Error("Error accepting incoming client connection", zap.Error(err))
				continue
			}
		}

		go s.handleClient(conn)
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
	}
	s.mu.Unlock()

	defer func() {
		// Delete a client
		s.mu.Lock()
		delete(s.clients, c)
		delete(Clients, c)
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
		// Process received data
		s.processPacket(c, line)
	}
}

// processPacket processes received data
func (s *TCPAPRSServer) processPacket(client *TCPAPRSClient, packet string) {
	// Parse packet
	if strings.HasPrefix(packet, "user ") {
		s.handleLogin(client, packet)
	} else if strings.HasPrefix(packet, "#") {
		s.handleComment(client)
	} else if strings.Contains(packet, ">") {
		s.handleAPRSData(client, packet)
	} else {
		_ = client.Send("# invalid packet")
	}
}

// handleLogin handles login command
func (s *TCPAPRSServer) handleLogin(client *TCPAPRSClient, packet string) {
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
			client.filter = parts[k+1]
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
		client.mu.Unlock()

		_ = client.Send(fmt.Sprintf("# logresp %s verified, server %s", callSign, config.C.GetString("server.id")))
		logger.L.Debug(fmt.Sprintf("Client logged in as %s", callSign), zap.String("client", client.conn.RemoteAddr().String()))
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
func (s *TCPAPRSServer) handleAPRSData(client *TCPAPRSClient, packet string) {
	client.mu.Lock()
	defer client.mu.Unlock()

	if !client.verified {
		_ = client.Send("# invalid login")
		return
	}

	uplink.Stream.Write(packet, client)

	fmt.Printf("APRS data from %s: %s\n", client.callSign, packet)
}

// handleUplinkData sends data to client
func (c *TCPAPRSClient) handleUplinkData() {
	for data := range c.dataCh {
		c.mu.Lock()
		if c.loggedIn && c.conn != nil {
			switch c.mode {
			case client.Fullfeed:
				_ = c.Send(data.Data)
			case client.IGate:
				if c.filter != "" {
					if data.Writer != c {
						_ = c.Send(data.Data)
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
