// Package security implements the configurable safety policies: callsign
// blacklists (login and source), the unverified-client relay restriction, and
// helpers shared by the inbound paths.
package security

import (
	"path"
	"strings"

	"github.com/APRSCN/aprsgo/internal/infra/config"
)

// builtinBogusSourceCalls are source callsigns that are never allowed to
// originate traffic, regardless of configuration.
var builtinBogusSourceCalls = []string{"N0CALL", "NOCALL", "SERVER"}

// globMatch reports whether name matches the shell-style glob pattern,
// case-insensitively (supporting * and ?).
func globMatch(pattern, name string) bool {
	ok, err := path.Match(strings.ToUpper(pattern), strings.ToUpper(name))
	return err == nil && ok
}

// LoginAllowed reports whether a login callsign is permitted (not matched by
// any disallow_login_call glob).
func LoginAllowed(callsign string) bool {
	for _, pat := range config.Get().Server.DisallowLoginCall {
		if globMatch(pat, callsign) {
			return false
		}
	}
	return true
}

// SourceAllowed reports whether a packet source callsign is permitted. The
// built-in bogus list always applies; the configured disallow_source_call
// globs are checked in addition.
func SourceAllowed(srccall string) bool {
	up := strings.ToUpper(strings.TrimSpace(srccall))
	if up == "" {
		return false
	}
	// Compare against built-ins on the SSID-stripped root as well.
	root := up
	if i := strings.IndexByte(root, '-'); i >= 0 {
		root = root[:i]
	}
	for _, b := range builtinBogusSourceCalls {
		if root == b {
			return false
		}
	}
	for _, pat := range config.Get().Server.DisallowSourceCall {
		if globMatch(pat, up) {
			return false
		}
	}
	return true
}

// DisallowUnverified reports whether unverified clients are barred from
// relaying packets.
func DisallowUnverified() bool {
	return config.Get().Server.DisallowUnverified
}

// QProtocolID returns the configured accepted q-construct protocol id letter,
// defaulting to "A".
func QProtocolID() string {
	id := strings.TrimSpace(config.Get().Server.QProtocolID)
	if id == "" {
		return "A"
	}
	return id
}

// DisallowOtherQProtocols reports whether q-constructs using a different
// protocol id should be dropped.
func DisallowOtherQProtocols() bool {
	return config.Get().Server.DisallowOtherQProtocols
}
