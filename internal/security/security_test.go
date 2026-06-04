package security

import (
	"testing"

	config2 "github.com/APRSCN/aprsgo/internal/infra/config"
)

func setSecurityConfig(login, source []string, unverified, disallowOtherQ bool, qid string) {
	var c config2.StaticConfig
	c.Server.DisallowLoginCall = login
	c.Server.DisallowSourceCall = source
	c.Server.DisallowUnverified = unverified
	c.Server.DisallowOtherQProtocols = disallowOtherQ
	c.Server.QProtocolID = qid
	config2.Set(c)
}

func TestLoginAllowed(t *testing.T) {
	setSecurityConfig([]string{"N0CALL", "TEST-*"}, nil, false, false, "")
	defer config2.Set(config2.StaticConfig{})

	cases := map[string]bool{
		"N0CALL":    false,
		"n0call":    false, // case-insensitive
		"TEST-1":    false, // glob
		"TEST-99":   false,
		"VALIDCALL": true,
	}
	for call, want := range cases {
		if got := LoginAllowed(call); got != want {
			t.Errorf("LoginAllowed(%s) = %v, want %v", call, got, want)
		}
	}
}

func TestSourceAllowed(t *testing.T) {
	setSecurityConfig(nil, []string{"BAD-*"}, false, false, "")
	defer config2.Set(config2.StaticConfig{})

	cases := map[string]bool{
		"N0CALL":   false, // built-in
		"NOCALL-1": false, // built-in root match (SSID stripped)
		"SERVER":   false, // built-in
		"BAD-7":    false, // configured glob
		"GOODCALL": true,
		"":         false, // empty
	}
	for call, want := range cases {
		if got := SourceAllowed(call); got != want {
			t.Errorf("SourceAllowed(%q) = %v, want %v", call, got, want)
		}
	}
}

func TestQProtocolDefaults(t *testing.T) {
	setSecurityConfig(nil, nil, false, false, "")
	defer config2.Set(config2.StaticConfig{})
	if QProtocolID() != "A" {
		t.Errorf("QProtocolID default = %q, want A", QProtocolID())
	}

	setSecurityConfig(nil, nil, true, true, "Z")
	if QProtocolID() != "Z" {
		t.Errorf("QProtocolID = %q, want Z", QProtocolID())
	}
	if !DisallowUnverified() {
		t.Error("DisallowUnverified should be true")
	}
	if !DisallowOtherQProtocols() {
		t.Error("DisallowOtherQProtocols should be true")
	}
}
