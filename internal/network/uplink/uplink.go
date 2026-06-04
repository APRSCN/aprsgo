package uplink

import (
	"fmt"
	"sync"
	"time"

	"github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"github.com/APRSCN/aprsgo/internal/meta"
	historydb2 "github.com/APRSCN/aprsgo/internal/pkg/historydb"
	"github.com/APRSCN/aprsgo/internal/wait"
	"github.com/APRSCN/aprsutils/client"
	"go.uber.org/zap"
)

// active holds the currently-connected uplink client per group (a group has at
// most one active link; multiple groups give parallel uplinks). It is read by
// the status handler and the per-group managers, so all access goes through the
// accessors below.
var (
	active   = make(map[string]*client.Client)
	clientMu sync.RWMutex
)

// uplink manager state. stopCh signals all manager/stats goroutines to exit;
// it is replaced (re-armed) by Init/Reload and read by the goroutines and Stop,
// so every access goes through stopMu.
var (
	stopMu   sync.Mutex
	stopCh   chan struct{}
	stopOnce sync.Once
	mgrWG    sync.WaitGroup
)

// currentStop returns the active stop channel under the lock. Goroutines call
// this once at startup so a concurrent re-arm (Init/Reload) cannot race the
// read of the package variable.
func currentStop() <-chan struct{} {
	stopMu.Lock()
	defer stopMu.Unlock()
	return stopCh
}

// armStop installs a fresh stop channel and resets the one-shot closer, under
// the lock. Called by Init and Reload before launching new goroutines.
func armStop() {
	stopMu.Lock()
	defer stopMu.Unlock()
	stopOnce = sync.Once{}
	stopCh = make(chan struct{})
}

// closeStop closes the active stop channel exactly once, under the lock.
func closeStop() {
	stopMu.Lock()
	defer stopMu.Unlock()
	stopOnce.Do(func() {
		if stopCh != nil {
			close(stopCh)
		}
	})
}

// reconnect backoff bounds.
const (
	minBackoff = 1 * time.Second
	maxBackoff = 30 * time.Second
)

// TCP keepalive parameters for uplink connections: probe after this idle time,
// at this interval, dropping the link after this many failed probes.
const (
	uplinkKeepAliveIdle     = 10 * time.Minute
	uplinkKeepAliveInterval = 20 * time.Second
	uplinkKeepAliveCount    = 3
)

// uplinkTarget is one configured uplink endpoint within a group.
type uplinkTarget struct {
	mode     string
	protocol string
	host     string
	port     int
}

// GetClient returns one active uplink client (any group), or nil if none is
// connected. Retained for the single-link status view.
func GetClient() *client.Client {
	clientMu.RLock()
	defer clientMu.RUnlock()
	for _, c := range active {
		if c != nil {
			return c
		}
	}
	return nil
}

// Clients returns a snapshot of all currently-active uplink clients keyed by
// group name.
func Clients() map[string]*client.Client {
	clientMu.RLock()
	defer clientMu.RUnlock()
	out := make(map[string]*client.Client, len(active))
	for g, c := range active {
		if c != nil {
			out[g] = c
		}
	}
	return out
}

// setClient publishes (or clears, when c is nil) the active client for a group.
func setClient(group string, c *client.Client) {
	clientMu.Lock()
	if c == nil {
		delete(active, group)
	} else {
		active[group] = c
	}
	clientMu.Unlock()
}

// groupedUplinks partitions the configured uplinks by group name (empty group
// name maps to the "default" group), preserving order within each group.
func groupedUplinks() map[string][]uplinkTarget {
	groups := make(map[string][]uplinkTarget)
	for _, up := range config.Get().Server.Uplinks {
		g := up.Group
		if g == "" {
			g = "default"
		}
		groups[g] = append(groups[g], uplinkTarget{
			mode: up.Mode, protocol: up.Protocol, host: up.Host, port: up.Port,
		})
	}
	return groups
}

// Init inits uplink daemon
func Init() {
	// Init Stream
	Stream = NewDataStream(100)

	// Init dupRecords
	dupRecords = historydb2.NewDupeChecker(time.Second)

	// Init stats
	StatsPacketRX = historydb2.NewMapFloat64History()
	StatsPacketTX = historydb2.NewMapFloat64History()
	StatsBytesRX = historydb2.NewMapFloat64History()
	StatsBytesTX = historydb2.NewMapFloat64History()

	armStop()
	startManagers()

	// Start stats
	mgrWG.Add(2)
	go rate()
	go stats()

	logger.L.Debug("Uplink daemon initialized")
}

// startManagers launches one manager goroutine per configured uplink group. If
// no uplinks are configured a single idle manager runs so the daemon still
// honours stop/reload cleanly.
func startManagers() {
	groups := groupedUplinks()
	if len(groups) == 0 {
		mgrWG.Add(1)
		go manageGroup("default", nil)
		return
	}
	for name, targets := range groups {
		mgrWG.Add(1)
		go manageGroup(name, targets)
	}
}

// manageGroup runs the connection lifecycle for one uplink group: it rotates
// through the group's targets, connecting to the first that answers, pumping
// the distribution stream to it, and reconnecting with exponential backoff when
// the link drops. At most one link in the group is active at a time.
func manageGroup(group string, targets []uplinkTarget) {
	defer mgrWG.Done()

	stop := currentStop() // capture locally; Reload re-arms the package var
	backoff := minBackoff

	for {
		select {
		case <-stop:
			return
		default:
		}

		if len(targets) == 0 {
			// Nothing configured for this group; idle until stopped.
			if !wait.SleepOrStop(stop, maxBackoff) {
				return
			}
			continue
		}

		connected := false
		for _, up := range targets {
			select {
			case <-stop:
				return
			default:
			}

			if tryUplink(group, up) {
				connected = true
				backoff = minBackoff // reset backoff after a good session
				break
			}
		}

		if !connected {
			logger.L.Warn("All uplinks in group unavailable, backing off",
				zap.String("group", group), zap.Duration("backoff", backoff))
			if !wait.SleepOrStop(stop, backoff) {
				return
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// tryUplink connects to a single uplink and, on success, blocks until the link
// drops. It returns true if a connection was established (regardless of how it
// later ended), false if the initial connect failed.
func tryUplink(group string, up uplinkTarget) bool {
	opts := []client.Option{
		client.WithBufSize(config.Get().Server.BuffSize * 1024),
		client.WithLogger(&ZapLogger{logger: logger.L}),
		client.WithSoftwareAndVersion(
			meta.ENName, fmt.Sprintf("%s/%s", meta.Nickname, meta.Version),
		),
		client.WithHandler(recvHandler),
		// Reconnection contract: the manager owns reconnection, so the
		// client's internal retry is disabled (WithRetryTimes(0)). With retry
		// disabled the client does not reconnect itself; instead, when the
		// link drops it releases c.Wait() (see client.Wait / signalDone). The
		// manager blocks on Wait() below and, once it returns, loops to dial a
		// fresh connection. This keeps a single source of truth for the link
		// lifecycle and avoids two competing reconnect loops.
		client.WithRetryTimes(0),
		// Detect a dead but idle upstream via TCP keepalive.
		client.WithKeepAlive(uplinkKeepAliveIdle, uplinkKeepAliveInterval, uplinkKeepAliveCount),
	}
	// Apply the configured upstream idle timeout, if any.
	if t := config.Get().Server.UpstreamTimeout; t > 0 {
		opts = append(opts, client.WithReadTimeout(time.Duration(t)*time.Second))
	}
	// Bind a local source address if configured.
	if v4, v6 := config.Get().Server.UplinkBindV4, config.Get().Server.UplinkBindV6; v4 != "" || v6 != "" {
		opts = append(opts, client.WithLocalAddr(v4, v6))
	}

	c := client.NewClient(
		config.Get().Server.ID,
		config.Get().Server.Passcode,
		client.Mode(up.mode), client.Protocol(up.protocol),
		up.host, up.port,
		opts...,
	)

	if err := c.Connect(); err != nil {
		logger.L.Debug("Uplink connect failed", zap.String("group", group),
			zap.String("host", up.host), zap.Int("port", up.port), zap.Error(err))
		return false
	}

	setClient(group, c)
	logger.L.Info("Uplink connected", zap.String("group", group),
		zap.String("host", up.host), zap.Int("port", up.port), zap.String("mode", up.mode))

	// Pump the distribution stream to this uplink for the duration of the link.
	ch, closeFn := Stream.Subscribe()
	go sendHandler(c, ch)

	// Wait until the client is closed (by remote drop or shutdown).
	c.Wait()

	closeFn()
	setClient(group, nil)
	logger.L.Info("Uplink disconnected", zap.String("group", group),
		zap.String("host", up.host), zap.Int("port", up.port))
	return true
}

// Reload restarts the uplink managers after a configuration change (e.g.
// SIGHUP), so new uplink targets take effect.
func Reload() {
	Stop()

	// Re-arm and start fresh managers plus the stats goroutines (all of which
	// exited when the stop channel was closed by Stop). All are tracked by
	// mgrWG so Stop waits for them before the next Reload re-arms.
	armStop()
	startManagers()
	mgrWG.Add(2)
	go rate()
	go stats()
	logger.L.Info("Uplink reloaded")
}

// Stop terminates the uplink managers and closes every active client.
func Stop() {
	closeStop()
	for _, c := range Clients() {
		c.Close()
	}
	mgrWG.Wait()
}
