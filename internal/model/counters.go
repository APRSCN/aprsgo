package model

import "sync/atomic"

// Counters holds the concurrently-updated cumulative statistics for a client,
// listener, server or the global aggregate. The cumulative totals are atomic so
// they can be incremented from many goroutines without locking. The derived
// per-second rates are computed by a single updater goroutine and stored in
// plain fields; readers obtain a consistent view via Snapshot.
type Counters struct {
	// Cumulative totals (atomic; incremented from many goroutines).
	sentPackets     atomic.Uint64
	receivedPackets atomic.Uint64
	receivedDups    atomic.Uint64
	receivedErrors  atomic.Uint64
	receivedQDrop   atomic.Uint64
	sentBytes       atomic.Uint64
	receivedBytes   atomic.Uint64

	// Per-second rates. Written only by UpdateRates (a single goroutine) and
	// read by Snapshot from other goroutines, so they are atomic.
	sendPacketRate atomic.Uint64
	recvPacketRate atomic.Uint64
	sendByteRate   atomic.Uint64
	recvByteRate   atomic.Uint64

	// Rate-calculation bookkeeping (only touched by UpdateRates).
	lastSentPackets     uint64
	lastReceivedPackets uint64
	lastSentBytes       uint64
	lastReceivedBytes   uint64
}

// AddSentPackets atomically adds to the sent-packet total.
func (c *Counters) AddSentPackets(n uint64) { c.sentPackets.Add(n) }

// AddReceivedPackets atomically adds to the received-packet total.
func (c *Counters) AddReceivedPackets(n uint64) { c.receivedPackets.Add(n) }

// AddReceivedDups atomically adds to the duplicate total.
func (c *Counters) AddReceivedDups(n uint64) { c.receivedDups.Add(n) }

// AddReceivedErrors atomically adds to the error total.
func (c *Counters) AddReceivedErrors(n uint64) { c.receivedErrors.Add(n) }

// AddReceivedQDrop atomically adds to the q-drop total.
func (c *Counters) AddReceivedQDrop(n uint64) { c.receivedQDrop.Add(n) }

// AddSentBytes atomically adds to the sent-byte total.
func (c *Counters) AddSentBytes(n uint64) { c.sentBytes.Add(n) }

// AddReceivedBytes atomically adds to the received-byte total.
func (c *Counters) AddReceivedBytes(n uint64) { c.receivedBytes.Add(n) }

// UpdateRates recomputes the per-second rates from the cumulative totals. It
// must be called by a single goroutine (e.g. a 1s ticker).
func (c *Counters) UpdateRates() {
	sp := c.sentPackets.Load()
	rp := c.receivedPackets.Load()
	sb := c.sentBytes.Load()
	rb := c.receivedBytes.Load()

	c.sendPacketRate.Store(sp - c.lastSentPackets)
	c.recvPacketRate.Store(rp - c.lastReceivedPackets)
	c.sendByteRate.Store(sb - c.lastSentBytes)
	c.recvByteRate.Store(rb - c.lastReceivedBytes)

	c.lastSentPackets = sp
	c.lastReceivedPackets = rp
	c.lastSentBytes = sb
	c.lastReceivedBytes = rb
}

// Snapshot returns a plain-value copy of the current statistics suitable for
// JSON serialisation and display.
func (c *Counters) Snapshot() Statistics {
	return Statistics{
		SentPackets:     c.sentPackets.Load(),
		ReceivedPackets: c.receivedPackets.Load(),
		ReceivedDups:    c.receivedDups.Load(),
		ReceivedErrors:  c.receivedErrors.Load(),
		ReceivedQDrop:   c.receivedQDrop.Load(),
		SentBytes:       c.sentBytes.Load(),
		ReceivedBytes:   c.receivedBytes.Load(),
		SendPacketRate:  c.sendPacketRate.Load(),
		RecvPacketRate:  c.recvPacketRate.Load(),
		SendByteRate:    c.sendByteRate.Load(),
		RecvByteRate:    c.recvByteRate.Load(),
	}
}
