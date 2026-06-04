package listener

import "testing"

func TestIsNonAPRSProbe(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
		desc string
	}{
		{"user N0CALL pass 12345 vers sw 1.0", false, "valid APRS login"},
		{"# comment line", false, "comment"},
		{"GET / HTTP/1.1", true, "HTTP GET"},
		{"POST /api HTTP/1.1", true, "HTTP POST"},
		{"OPTIONS * HTTP/1.0", true, "HTTP OPTIONS"},
		{"something HTTP/1.1", true, "embedded HTTP/"},
		{string([]byte{0x16, 0x03, 0x01, 0x00}), true, "TLS ClientHello"},
		{string([]byte{0x00, 0x01}), true, "binary leading byte"},
		{"SRC>DST,qAR,N0CALL:>hello", false, "APRS packet line"},
		{"", false, "empty"},
	}
	for _, c := range cases {
		if got := isNonAPRSProbe(c.raw); got != c.want {
			t.Errorf("%s: isNonAPRSProbe(%q) = %v, want %v", c.desc, c.raw, got, c.want)
		}
	}
}
