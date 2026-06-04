package config

type StaticConfig struct {
	Server struct {
		// Buff size setting (KB)
		BuffSize int `mapstructure:"buff_size"`
		// Custom processor name
		// Only available when we can not get CPU info
		Model string `mapstructure:"model"`
		// Your server ID
		ID string `mapstructure:"id"`
		// Passcode for the server ID
		Passcode string `mapstructure:"passcode"`

		// Global maximum number of simultaneous clients across all listeners.
		// 0 means unlimited.
		MaxClients int `mapstructure:"max_clients"`
		// Idle disconnect timeout for connected clients, in seconds.
		// 0 keeps the built-in default.
		ClientTimeout int `mapstructure:"client_timeout"`
		// Time a connection may stay open without logging in, in seconds.
		// 0 keeps the built-in default.
		LoginTimeout int `mapstructure:"login_timeout"`
		// Idle disconnect timeout for the upstream uplink, in seconds.
		// 0 keeps the built-in default.
		UpstreamTimeout int `mapstructure:"upstream_timeout"`

		// Security toggles.
		// DisallowUnverified drops relayed packets from clients that did not
		// authenticate with a valid passcode (they may still query the server
		// and update their own position).
		DisallowUnverified bool `mapstructure:"disallow_unverified"`
		// DisallowLoginCall rejects logins whose callsign matches any of these
		// glob patterns (case-insensitive).
		DisallowLoginCall []string `mapstructure:"disallow_login_call"`
		// DisallowSourceCall drops packets whose source callsign matches any of
		// these glob patterns (case-insensitive), in addition to the built-in
		// list of bogus callsigns.
		DisallowSourceCall []string `mapstructure:"disallow_source_call"`
		// DisallowOtherQProtocols drops packets whose q-construct protocol id
		// differs from QProtocolID.
		DisallowOtherQProtocols bool `mapstructure:"disallow_other_q_protocols"`
		// QProtocolID is the accepted q-construct protocol id letter (default
		// "A").
		QProtocolID string `mapstructure:"q_protocol_id"`

		// UplinkBindV4 / UplinkBindV6 optionally bind the local source address
		// for outbound uplink connections, chosen by the remote address family.
		// Empty lets the OS choose.
		UplinkBindV4 string `mapstructure:"uplink_bind_v4"`
		UplinkBindV6 string `mapstructure:"uplink_bind_v6"`

		// Setting of http status panel
		Status struct {
			Host string `mapstructure:"host"`
			Port int    `mapstructure:"port"`
		} `mapstructure:"status"`
		// Setting of aprs server
		// Mode: fullfeed [Everything] / igate [IGate / Client Port] /
		// dupefeed [Everything incl. duplicates, receive-only]
		Listeners []ListenerConfig `mapstructure:"listeners"`
		Uplinks   []struct {
			Name     string `mapstructure:"name"`
			Mode     string `mapstructure:"mode"`
			Protocol string `mapstructure:"protocol"`
			Host     string `mapstructure:"host"`
			Port     int    `mapstructure:"port"`
			// Group ties alternative uplinks together: within a group exactly
			// one link is kept active (the others are failover alternatives and
			// are tried in rotation). Different groups each maintain their own
			// active link in parallel, giving multiple simultaneous uplinks.
			// Empty group name means the default group.
			Group string `mapstructure:"group"`
		} `mapstructure:"uplinks"`
		// Core peers: UDP server-to-server links exchanging raw APRS lines.
		// 'peer' is kept for backwards compatibility (a single group); use
		// 'peergroups' for multiple independent mesh groups.
		Peer       PeerGroupConfig   `mapstructure:"peer"`
		PeerGroups []PeerGroupConfig `mapstructure:"peergroups"`
	} `mapstructure:"server"`
	// Info of server admin
	Admin struct {
		Name  string `mapstructure:"name"`
		Email string `mapstructure:"email"`
	} `mapstructure:"admin"`
	// Config of system log
	Log struct {
		File       string `mapstructure:"file"`
		MaxSize    int    `mapstructure:"max_size"`
		MaxBackups int    `mapstructure:"max_backups"`
		MaxAge     int    `mapstructure:"max_age"`
		Compress   bool   `mapstructure:"compress"`
	} `mapstructure:"log"`
}

// ListenerConfig describes a single inbound listener.
type ListenerConfig struct {
	Name     string `mapstructure:"name"`
	Mode     string `mapstructure:"mode"`
	Protocol string `mapstructure:"protocol"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Visible  string `mapstructure:"visible"`
	Filter   string `mapstructure:"filter"`
	// TLS: when enabled, a tcp listener serves TLS (APRS-IS over TLS).
	// Requires Cert and Key (PEM file paths).
	TLS  bool   `mapstructure:"tls"`
	Cert string `mapstructure:"cert"`
	Key  string `mapstructure:"key"`
	// ClientCA is an optional PEM file of CA certificates used to verify
	// client certificates. When set, a client presenting a certificate issued
	// by this CA whose callsign matches its login is accepted as verified
	// without a passcode.
	ClientCA string `mapstructure:"client_ca"`

	// MaxClients caps the simultaneous clients on this listener (0 =
	// unlimited / use the global cap only).
	MaxClients int `mapstructure:"max_clients"`
	// IBufSize / OBufSize override the global buffer size (KB) for this
	// listener's input reader and output queue (0 = use global buff_size).
	IBufSize int `mapstructure:"ibuf_size"`
	OBufSize int `mapstructure:"obuf_size"`
	// ACL is an ordered list of access-control rules, each "allow <CIDR>" or
	// "deny <CIDR>". When non-empty the default policy is denied.
	ACL []string `mapstructure:"acl"`
}

// PeerGroupConfig describes one core-peer mesh group: a local UDP bind address
// plus the set of remote peers it exchanges traffic with.
type PeerGroupConfig struct {
	Name string `mapstructure:"name"`
	// Local UDP listen address for inbound peer datagrams.
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	// Remote peers to exchange traffic with.
	Peers []PeerConfig `mapstructure:"peers"`
}

// PeerConfig is a single remote peer within a group.
type PeerConfig struct {
	Name string `mapstructure:"name"`
	ID   string `mapstructure:"id"`
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	// Protocol selects this peer's transport: "udp" (default) or "tcp". Peers
	// within a group may mix transports.
	Protocol string `mapstructure:"protocol"`
}
