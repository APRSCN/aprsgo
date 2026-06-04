package listener

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"net"
	"strings"
)

// callsignOID is the object identifier carrying the operator callsign in
// amateur-radio client certificates (the TrustedQSL/LoTW convention). When
// present it is preferred over the certificate's Common Name.
var callsignOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 12348, 1, 1}

// rootCall returns the callsign with any SSID suffix removed, upper-cased and
// trimmed, for case-insensitive comparison.
func rootCall(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if i := strings.IndexByte(s, '-'); i >= 0 {
		s = s[:i]
	}
	return s
}

// callsignFromCert extracts the operator callsign from a client certificate:
// the dedicated callsign OID if present, otherwise the Subject Common Name.
func callsignFromCert(cert *x509.Certificate) string {
	for _, n := range cert.Subject.Names {
		if n.Type.Equal(callsignOID) {
			if s, ok := n.Value.(string); ok && s != "" {
				return s
			}
		}
	}
	return cert.Subject.CommonName
}

// verifiedByCert reports whether the connection is a TLS connection presenting
// a verified client certificate whose callsign matches the login callsign
// (ignoring SSID and case). It returns false for non-TLS connections or when no
// verified certificate was supplied.
func verifiedByCert(conn net.Conn, loginCall string) bool {
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return false
	}
	st := tlsConn.ConnectionState()
	// VerifiedChains is non-empty only when the presented certificate chained
	// to a configured client CA and passed verification during the handshake.
	if len(st.VerifiedChains) == 0 || len(st.PeerCertificates) == 0 {
		return false
	}
	want := rootCall(loginCall)
	if want == "" {
		return false
	}
	return rootCall(callsignFromCert(st.PeerCertificates[0])) == want
}
