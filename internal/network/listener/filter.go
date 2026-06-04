package listener

import (
	"github.com/APRSCN/aprsgo/internal/pkg/historydb"
	"github.com/APRSCN/aprsutils/filter"
)

// filterContext implements filter.Context for a connected client, resolving
// station positions from the shared position history and the client's own
// last-known position for the m/ filter.
type filterContext struct {
	clientCall string
}

// newFilterContext builds a Context bound to the given client callsign.
func newFilterContext(clientCall string) filter.Context {
	return &filterContext{clientCall: clientCall}
}

// ClientPosition returns the client's own last-known position, looked up by its
// login callsign in the position history.
func (c *filterContext) ClientPosition() (filter.Position, bool) {
	if c.clientCall == "" {
		return filter.Position{}, false
	}
	lat, lon, ok := historydb.Positions.Get(c.clientCall)
	if !ok {
		return filter.Position{}, false
	}
	return filter.Position{Lat: lat, Lon: lon}, true
}

// StationPosition returns an arbitrary station's last-known position.
func (c *filterContext) StationPosition(call string) (filter.Position, bool) {
	lat, lon, ok := historydb.Positions.Get(call)
	if !ok {
		return filter.Position{}, false
	}
	return filter.Position{Lat: lat, Lon: lon}, true
}
