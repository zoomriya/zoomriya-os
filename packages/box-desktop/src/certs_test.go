package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// bundle writes a certificate, and its key, for these names, as the agent
// and connectd write them.
func bundle(t *testing.T, path string, names []string, ips []net.IP, notAfter time.Time) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: names[0]},
		DNSNames: names, IPAddresses: ips, NotBefore: time.Now().Add(-time.Hour), NotAfter: notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalECPrivateKey(key)
	out := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	out = append(out, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})...)
	if err := os.WriteFile(path, out, 0o640); err != nil {
		t.Fatal(err)
	}
}

func TestCertificatesByName(t *testing.T) {
	dir := t.TempDir()
	year := time.Now().Add(365 * 24 * time.Hour)
	bundle(t, filepath.Join(dir, "box.pem"), []string{"maplehouse.local"}, []net.IP{net.ParseIP("192.168.1.10")}, year)
	bundle(t, filepath.Join(dir, "connect.pem"), []string{"maplehouse.zoomriya.app"}, nil, year)
	bundle(t, filepath.Join(dir, "old.pem"), []string{"old.zoomriya.app"}, nil, time.Now().Add(-time.Minute))
	c := NewCertificates(dir)

	for name, want := range map[string]string{
		"maplehouse.zoomriya.app": "maplehouse.zoomriya.app",
		"maplehouse.local":        "maplehouse.local",
		"unknown.example":         "maplehouse.local", // the box's own answers the rest
	} {
		cert, err := c.Get(&tls.ClientHelloInfo{ServerName: name})
		if err != nil || cert.Leaf.DNSNames[0] != want {
			t.Errorf("%s: %v %v", name, err, cert)
		}
	}
	for host, want := range map[string]bool{
		"maplehouse.zoomriya.app": true, "MapleHouse.local": true, "192.168.1.10": true,
		"old.zoomriya.app": false, "evil.example": false,
	} {
		if c.Names(host) != want {
			t.Errorf("Names(%s) = %v", host, !want)
		}
	}
}
