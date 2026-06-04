// Package acl implements ordered IP/CIDR access-control lists. Each rule is
// either "allow <CIDR>" or "deny <CIDR>"; rules are evaluated in order and the
// first matching rule decides. When a list has any rules, the default policy
// is denied (an address matching no rule is rejected). An empty/nil list allows
// everything, preserving the previous open behaviour.
package acl

import (
	"fmt"
	"net/netip"
	"strings"
)

// rule is a single parsed access-control entry.
type rule struct {
	allow  bool
	prefix netip.Prefix
}

// List is a compiled, ordered access-control list. A nil *List allows all.
type List struct {
	rules []rule
}

// Compile parses a slice of "allow <CIDR>" / "deny <CIDR>" strings into a List.
// A nil or empty input yields a nil List (allow all). Invalid entries produce
// an error and the offending rule text.
func Compile(specs []string) (*List, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	l := &List{}
	for _, spec := range specs {
		fields := strings.Fields(spec)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid acl rule %q: want \"allow|deny <CIDR>\"", spec)
		}
		var allow bool
		switch strings.ToLower(fields[0]) {
		case "allow":
			allow = true
		case "deny":
			allow = false
		default:
			return nil, fmt.Errorf("invalid acl action %q in rule %q", fields[0], spec)
		}
		prefix, err := netip.ParsePrefix(fields[1])
		if err != nil {
			return nil, fmt.Errorf("invalid acl CIDR %q: %w", fields[1], err)
		}
		l.rules = append(l.rules, rule{allow: allow, prefix: prefix.Masked()})
	}
	return l, nil
}

// Allow reports whether the given address string (an IP, optionally an
// "ip:port" or "[ip]:port") is permitted. A nil list allows everything.
func (l *List) Allow(addr string) bool {
	if l == nil || len(l.rules) == 0 {
		return true
	}
	ip, ok := parseIP(addr)
	if !ok {
		// Unparseable address: reject under an active policy.
		return false
	}
	return l.AllowAddr(ip)
}

// AllowAddr reports whether the given address is permitted. A nil list allows
// everything. IPv4-mapped IPv6 addresses are normalised to IPv4 so rules
// written for IPv4 also match mapped connections.
func (l *List) AllowAddr(ip netip.Addr) bool {
	if l == nil || len(l.rules) == 0 {
		return true
	}
	ip = ip.Unmap()
	for _, r := range l.rules {
		if prefixContains(r.prefix, ip) {
			return r.allow
		}
	}
	// Default policy under an active list is denied.
	return false
}

// prefixContains reports whether prefix p contains ip, treating an
// IPv4-mapped prefix and a plain IPv4 address (and vice versa) as comparable.
func prefixContains(p netip.Prefix, ip netip.Addr) bool {
	pa := p.Addr().Unmap()
	if pa.Is4() != ip.Is4() {
		return false
	}
	return netip.PrefixFrom(pa, p.Bits()).Contains(ip)
}

// parseIP extracts a netip.Addr from a bare IP, "ip:port" or "[ip]:port".
func parseIP(addr string) (netip.Addr, bool) {
	if ip, err := netip.ParseAddr(addr); err == nil {
		return ip, true
	}
	if ap, err := netip.ParseAddrPort(addr); err == nil {
		return ap.Addr(), true
	}
	// Last resort: strip a trailing ":port" for host:port forms not covered
	// above.
	if i := strings.LastIndexByte(addr, ':'); i >= 0 {
		if ip, err := netip.ParseAddr(strings.Trim(addr[:i], "[]")); err == nil {
			return ip, true
		}
	}
	return netip.Addr{}, false
}
