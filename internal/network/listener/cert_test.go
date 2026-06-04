package listener

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

// TestRootCall checks SSID stripping and case folding.
func TestRootCall(t *testing.T) {
	cases := map[string]string{
		"N0CALL-9": "N0CALL",
		" n0call ": "N0CALL",
		"AB1CD":    "AB1CD",
	}
	for in, want := range cases {
		if got := rootCall(in); got != want {
			t.Errorf("rootCall(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestCallsignFromCert verifies the callsign is taken from the dedicated OID
// when present, otherwise from the Common Name.
func TestCallsignFromCert(t *testing.T) {
	cnOnly := &x509.Certificate{Subject: pkix.Name{CommonName: "AB1CD"}}
	if got := callsignFromCert(cnOnly); got != "AB1CD" {
		t.Errorf("CN cert callsign = %q, want AB1CD", got)
	}

	withOID := &x509.Certificate{Subject: pkix.Name{
		CommonName: "Ignore Me",
		Names:      []pkix.AttributeTypeAndValue{{Type: callsignOID, Value: "XY2ZZ"}},
	}}
	if got := callsignFromCert(withOID); got != "XY2ZZ" {
		t.Errorf("OID cert callsign = %q, want XY2ZZ", got)
	}
}

// genCA creates a self-signed CA certificate and key.
func genCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert, key
}

// genLeaf creates a certificate signed by the CA carrying the given callsign in
// its Common Name.
func genLeaf(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, call string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: call},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// TestVerifiedByCert performs a real TLS handshake with a client certificate
// and checks the callsign-match verification on the server side.
func TestVerifiedByCert(t *testing.T) {
	ca, caKey := genCA(t)
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serverTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "server"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTmpl, ca, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	serverCert := tls.Certificate{Certificate: [][]byte{serverDER}, PrivateKey: serverKey}

	caPool := x509.NewCertPool()
	caPool.AddCert(ca)

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    caPool,
		ClientAuth:   tls.VerifyClientCertIfGiven,
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	type result struct {
		ok   bool
		call string
	}
	resCh := make(chan result, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			resCh <- result{}
			return
		}
		defer conn.Close()
		// Force the handshake to complete so peer certificates are available.
		if tc, ok := conn.(*tls.Conn); ok {
			_ = tc.Handshake()
		}
		resCh <- result{ok: verifiedByCert(conn, "AB1CD-7"), call: "AB1CD-7"}
	}()

	clientCert := genLeaf(t, ca, caKey, "AB1CD")
	c, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{
		// The test only exercises server-side client-certificate verification;
		// skip server name/chain verification on the client side.
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{clientCert},
	})
	if err != nil {
		t.Fatalf("client dial: %v", err)
	}
	if err := c.Handshake(); err != nil {
		t.Fatalf("client handshake: %v", err)
	}

	select {
	case r := <-resCh:
		if !r.ok {
			t.Error("verifiedByCert = false, want true for matching cert callsign (SSID ignored)")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not finish verification")
	}
	c.Close()
}
