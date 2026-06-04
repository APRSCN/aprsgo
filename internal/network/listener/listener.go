package listener

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/internal/pkg/acl"
	"github.com/APRSCN/aprsutils/client"
	"github.com/APRSCN/aprsutils/filter"
	"go.uber.org/zap"
)

// errSCTPUnsupported is returned when SCTP is requested on a platform without
// SCTP support.
var errSCTPUnsupported = errors.New("SCTP is only supported on Linux")

// Listener provides a struct to record listener
type Listener struct {
	Name     string
	Type     string
	Protocol string
	Host     string
	Port     int
	Visible  string
	Filter   string

	// onlineClient / peakClient track the current and peak simultaneous client
	// counts. They are written by register/unregister and read by the status
	// handler from other goroutines, so they are atomic.
	onlineClient atomic.Int64
	peakClient   atomic.Int64

	// compiledFilter is the listener-level filter (compiled once from Filter).
	compiledFilter *filter.Filter

	// acl is the compiled access-control list (nil = allow all).
	acl *acl.List

	// maxClients caps simultaneous clients on this listener (0 = no per-port
	// cap; the global cap still applies).
	maxClients int
	// ibufBytes / obufBytes are the input reader and output queue sizes in
	// bytes (already resolved from config or the global default).
	ibufBytes int
	obufBytes int
	// dupefeed marks a receive-only port that also delivers duplicate packets.
	dupefeed bool

	s  *TCPAPRSServer   // TCP server (nil for UDP listeners)
	us *UDPSubmitServer // UDP submit server (nil for TCP listeners)

	// stats holds the latest statistics snapshot. It is stored behind an atomic
	// pointer so the rate-updater goroutine can publish a new snapshot while the
	// status HTTP handler reads concurrently without a data race.
	stats atomic.Pointer[model.Statistics]
}

// SetStats atomically publishes a new statistics snapshot for this listener.
func (l *Listener) SetStats(s model.Statistics) {
	l.stats.Store(&s)
}

// Stats returns the latest statistics snapshot (zero value if none published).
func (l *Listener) Stats() model.Statistics {
	if p := l.stats.Load(); p != nil {
		return *p
	}
	return model.Statistics{}
}

// OnlineClient returns the current number of connected clients.
func (l *Listener) OnlineClient() int { return int(l.onlineClient.Load()) }

// PeakClient returns the peak number of simultaneous clients observed.
func (l *Listener) PeakClient() int { return int(l.peakClient.Load()) }

// setOnlineClient publishes the current client count and bumps the peak.
func (l *Listener) setOnlineClient(n int) {
	l.onlineClient.Store(int64(n))
	for {
		peak := l.peakClient.Load()
		if int64(n) <= peak || l.peakClient.CompareAndSwap(peak, int64(n)) {
			return
		}
	}
}

// stop shuts down whichever underlying server this listener owns.
func (l *Listener) stop() {
	if l.s != nil {
		l.s.Stop()
	}
	if l.us != nil {
		l.us.Stop()
	}
}

// Listeners records all listeners. It is replaced wholesale by load() (on
// startup and SIGHUP reload) and read by many goroutines, so all access must
// go through ListenersMutex.
var Listeners = make([]*Listener, 0)

// ListenersMutex guards the Listeners slice (its header), not the individual
// Listener structs, whose own fields are independently synchronised.
var ListenersMutex sync.RWMutex

// snapshotListeners returns a shallow copy of the Listeners slice taken under
// the read lock, so callers can iterate without holding the lock.
func snapshotListeners() []*Listener {
	ListenersMutex.RLock()
	defer ListenersMutex.RUnlock()
	out := make([]*Listener, len(Listeners))
	copy(out, Listeners)
	return out
}

// ListenersSnapshot is the exported, race-free way to obtain the current
// listener set (e.g. for the status handler).
func ListenersSnapshot() []*Listener { return snapshotListeners() }

// listenerAt returns the listener at index i, or nil if out of range, taken
// under the read lock.
func listenerAt(i int) *Listener {
	ListenersMutex.RLock()
	defer ListenersMutex.RUnlock()
	if i < 0 || i >= len(Listeners) {
		return nil
	}
	return Listeners[i]
}

// Init inits listener daemon
func Init() {
	// Load init config
	load()

	// Start update daemon
	go update()

	logger.L.Debug("Listener initialized")
}

// Reload rebuilds the listener set from the (already reloaded) configuration,
// stopping ports that changed/went away and starting new ones. Safe to call on
// SIGHUP.
func Reload() {
	load()
	logger.L.Info("Listeners reloaded")
}

// load (re)builds the listener set from config. It stops the previous
// listeners, constructs the new ones, publishes them under the write lock and
// only then starts their servers, so that goroutines started by Start() always
// observe the published slice (e.g. register/unregister via listenerAt).
func load() {
	// Close existing servers.
	for _, v := range snapshotListeners() {
		v.stop()
	}

	// Build the new listener set (without starting servers yet).
	globalBuf := config.Get().Server.BuffSize
	if globalBuf <= 0 {
		globalBuf = 128
	}
	built := make([]*Listener, 0)
	for _, lc := range config.Get().Server.Listeners {
		idx := len(built)

		// Precompile the listener-level filter (if any).
		var lf *filter.Filter
		if lc.Filter != "" {
			lf = filter.Compile(lc.Filter)
		}

		// Compile the access-control list (if any).
		al, err := acl.Compile(lc.ACL)
		if err != nil {
			logger.L.Error("Invalid ACL, listener disabled",
				zap.String("name", lc.Name), zap.Error(err))
			continue
		}

		// Resolve buffer sizes (KB -> bytes), falling back to the global size.
		ibuf := globalBuf
		if lc.IBufSize > 0 {
			ibuf = lc.IBufSize
		}
		obuf := globalBuf
		if lc.OBufSize > 0 {
			obuf = lc.OBufSize
		}

		l := &Listener{
			Name:           lc.Name,
			Type:           lc.Mode,
			Protocol:       lc.Protocol,
			Host:           lc.Host,
			Port:           lc.Port,
			Visible:        lc.Visible,
			Filter:         lc.Filter,
			compiledFilter: lf,
			acl:            al,
			maxClients:     lc.MaxClients,
			ibufBytes:      ibuf * 1024,
			obufBytes:      obuf * 1024,
			dupefeed:       lc.Mode == "dupefeed",
		}

		// A dupefeed port serves the duplicate stream; internally it behaves
		// like a fullfeed TCP server but is hidden and receive-only.
		mode := client.Mode(lc.Mode)
		if l.dupefeed {
			mode = client.Fullfeed
			if l.Visible == "" {
				l.Visible = "hidden"
			}
		}

		switch lc.Protocol {
		case "tcp":
			l.s = NewTCPAPRSServer(mode, idx)
			if lc.TLS {
				if err := l.s.SetTLS(lc.Cert, lc.Key, lc.ClientCA); err != nil {
					logger.L.Error("Error loading TLS cert/key, listener disabled",
						zap.String("name", lc.Name), zap.Error(err))
					continue
				}
			}
		case "udp":
			l.us = NewUDPSubmitServer(idx)
		case "sctp":
			l.s = NewTCPAPRSServer(mode, idx)
			if err := l.s.SetSCTP(); err != nil {
				logger.L.Error("SCTP unsupported, listener disabled",
					zap.String("name", lc.Name), zap.Error(err))
				continue
			}
		default:
			logger.L.Warn("Unsupported listener protocol, skipping",
				zap.String("name", lc.Name), zap.String("protocol", lc.Protocol))
			continue
		}
		built = append(built, l)
	}

	// Publish the new slice before starting servers so register/unregister
	// (which run on freshly started goroutines) see a consistent set.
	ListenersMutex.Lock()
	Listeners = built
	ListenersMutex.Unlock()

	// Start the servers now that the slice is published.
	for _, l := range built {
		addr := fmt.Sprintf("%s:%d", l.Host, l.Port)
		switch {
		case l.s != nil:
			if err := l.s.Start(addr); err != nil {
				logger.L.Error("Error starting server", zap.String("addr", addr), zap.Error(err))
			}
		case l.us != nil:
			if err := l.us.Start(addr); err != nil {
				logger.L.Error("Error starting UDP submit server", zap.String("addr", addr), zap.Error(err))
			}
		}
	}
}
