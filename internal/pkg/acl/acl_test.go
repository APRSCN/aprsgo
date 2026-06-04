package acl

import (
	"net/netip"
	"testing"
)

func TestNilListAllowsAll(t *testing.T) {
	var l *List
	if !l.Allow("203.0.113.5") {
		t.Fatal("nil list should allow all")
	}
	l2, err := Compile(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !l2.Allow("203.0.113.5") {
		t.Fatal("empty list should allow all")
	}
}

func TestAllowDenyOrder(t *testing.T) {
	l, err := Compile([]string{
		"allow 10.0.0.0/8",
		"deny 0.0.0.0/0",
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{
		"10.1.2.3":    true,  // matches allow
		"10.255.0.1":  true,  // matches allow
		"192.168.1.1": false, // falls to deny all
		"203.0.113.9": false, // falls to deny all
	}
	for addr, want := range cases {
		if got := l.Allow(addr); got != want {
			t.Errorf("Allow(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestDefaultDenyWhenRulesPresent(t *testing.T) {
	// A list with only an allow rule denies everything else by default.
	l, err := Compile([]string{"allow 192.0.2.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	if !l.Allow("192.0.2.50") {
		t.Error("address in allowed range should pass")
	}
	if l.Allow("198.51.100.1") {
		t.Error("address outside allowed range should be denied by default")
	}
}

func TestIPv6(t *testing.T) {
	l, err := Compile([]string{
		"allow 2001:db8::/32",
		"deny ::/0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !l.Allow("2001:db8::1") {
		t.Error("v6 in range should pass")
	}
	if l.Allow("2001:dead::1") {
		t.Error("v6 out of range should be denied")
	}
}

func TestIPv4MappedV6(t *testing.T) {
	l, err := Compile([]string{"allow 10.0.0.0/8", "deny 0.0.0.0/0"})
	if err != nil {
		t.Fatal(err)
	}
	mapped := netip.MustParseAddr("::ffff:10.1.2.3")
	if !l.AllowAddr(mapped) {
		t.Error("IPv4-mapped IPv6 should match IPv4 rule")
	}
}

func TestHostPortForms(t *testing.T) {
	l, _ := Compile([]string{"allow 10.0.0.0/8", "deny 0.0.0.0/0"})
	if !l.Allow("10.1.2.3:14580") {
		t.Error("ip:port should parse")
	}
	if !l.Allow("[2001:db8::1]:14580") {
		l2, _ := Compile([]string{"allow 2001:db8::/32", "deny ::/0"})
		if !l2.Allow("[2001:db8::1]:14580") {
			t.Error("[ipv6]:port should parse")
		}
	}
}

func TestInvalidRule(t *testing.T) {
	if _, err := Compile([]string{"permit 10.0.0.0/8"}); err == nil {
		t.Error("invalid action should error")
	}
	if _, err := Compile([]string{"allow notacidr"}); err == nil {
		t.Error("invalid CIDR should error")
	}
}
