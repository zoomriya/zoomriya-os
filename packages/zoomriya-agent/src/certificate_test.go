package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const certIPJSON = `[
 {"ifname":"lo","link_type":"loopback","address":"00:00:00:00:00:00","operstate":"UNKNOWN",
  "addr_info":[{"family":"inet","local":"127.0.0.1","prefixlen":8,"scope":"host"}]},
 {"ifname":"enp1s0","link_type":"ether","address":"52:54:00:12:34:56","operstate":"UP",
  "addr_info":[{"family":"inet","local":"%s","prefixlen":24,"scope":"global"},
               {"family":"inet6","local":"2001:db8::5","prefixlen":64,"scope":"global"},
               {"family":"inet6","local":"fe80::1","prefixlen":64,"scope":"link"}]}]`

func certAgent(t *testing.T) (*Agent, *string, *time.Time) {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "etc"), 0o755)
	os.WriteFile(filepath.Join(root, "etc/hostname"), []byte("zoomriya-7k2q\n"), 0o644)
	// linkKind looks for the hardware under sys/class/net.
	os.MkdirAll(filepath.Join(root, "sys/class/net/enp1s0/device"), 0o755)
	ip := "192.168.1.20"
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	agent := &Agent{Root: root, Now: func() time.Time { return now }}
	agent.Run = func(name string, args ...string) ([]byte, error) {
		if name == "ip" {
			return []byte(fmt.Sprintf(certIPJSON, ip)), nil
		}
		return nil, errors.New("unexpected " + name)
	}
	return agent, &ip, &now
}

func ownCert(t *testing.T, agent *Agent) *x509.Certificate {
	t.Helper()
	data, err := os.ReadFile(agent.path(tlsOwn))
	if err != nil {
		t.Fatal(err)
	}
	cert, err := firstCertificate(data)
	if err != nil {
		t.Fatal(err)
	}
	if !hasKey(data, cert) {
		t.Fatal("the bundle has no matching key")
	}
	return cert
}

func TestCertificateMadeAndKept(t *testing.T) {
	agent, ip, now := certAgent(t)
	n := &notes{}
	if err := agent.certificate(n); err != nil {
		t.Fatal(err)
	}
	if len(n.list) != 0 {
		t.Fatalf("nothing to tell: %+v", n.list)
	}
	cert := ownCert(t, agent)
	if strings.Join(cert.DNSNames, ",") != "zoomriya-7k2q.local" || len(cert.IPAddresses) != 1 || cert.IPAddresses[0].String() != "192.168.1.20" {
		t.Fatalf("names %v, addresses %v (only the home address)", cert.DNSNames, cert.IPAddresses)
	}

	// It verifies against the box's authority for its name and address.
	ca, _, err := agent.loadCA()
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	for _, name := range []string{"zoomriya-7k2q.local", "192.168.1.20"} {
		if _, err := cert.Verify(x509.VerifyOptions{DNSName: name, Roots: pool, CurrentTime: *now}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if info, _ := os.Stat(agent.path(tlsCAKey)); info.Mode().Perm() != 0o600 {
		t.Fatalf("authority's key is %v", info.Mode().Perm())
	}
	if info, _ := os.Stat(agent.path(tlsOwn)); info.Mode().Perm() != 0o640 {
		t.Fatalf("bundle is %v", info.Mode().Perm())
	}

	// What Control Panel › Security shows.
	status := agent.certificateStatus()
	if status["fingerprint"] != caFingerprint(ca) || status["trust_at"] != "http://zoomriya-7k2q.local/zoomriya-box.crt" ||
		status["until"] != cert.NotAfter.UTC().Format(time.RFC3339) || len(status["problems"].([]string)) != 0 {
		t.Fatalf("status: %+v", status)
	}

	// Nothing changed: the same certificate stays.
	if err := agent.certificate(n); err != nil || !ownCert(t, agent).Equal(cert) {
		t.Fatalf("remade without a reason (%v)", err)
	}

	// A new address: remade, by the same authority.
	*ip = "192.168.1.31"
	agent.certificate(n)
	again := ownCert(t, agent)
	if again.Equal(cert) || again.IPAddresses[0].String() != "192.168.1.31" || again.CheckSignatureFrom(ca) != nil {
		t.Fatalf("not remade for the new address: %v", again.IPAddresses)
	}

	// A month before it runs out: renewed.
	*now = again.NotAfter.Add(-20 * 24 * time.Hour)
	agent.certificate(n)
	if renewed := ownCert(t, agent); !renewed.NotAfter.After(again.NotAfter) {
		t.Fatal("not renewed")
	}
	if len(n.list) != 0 {
		t.Fatalf("renewing is quiet: %+v", n.list)
	}
}

// The authority can't vouch for anything on the internet.
func TestCertificateAuthorityIsLimited(t *testing.T) {
	agent, _, now := certAgent(t)
	if err := agent.certificate(&notes{}); err != nil {
		t.Fatal(err)
	}
	ca, caKey, _ := agent.loadCA()
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	for _, bad := range []struct {
		name string
		ip   net.IP
	}{{"zoomriya.com", nil}, {"", net.ParseIP("8.8.8.8")}} {
		key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		tmpl := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: "x"},
			NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		check := bad.name
		if bad.name != "" {
			tmpl.DNSNames = []string{bad.name}
		} else {
			tmpl.IPAddresses = []net.IP{bad.ip}
			check = bad.ip.String()
		}
		der, _ := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
		cert, _ := x509.ParseCertificate(der)
		if _, err := cert.Verify(x509.VerifyOptions{DNSName: check, Roots: pool, CurrentTime: *now}); err == nil {
			t.Fatalf("the box's authority vouched for %s", check)
		}
	}
}

// connectd's real certificate: told once when it's running out or ran out,
// and when it's fixed.
func TestCertificateWatchesOthers(t *testing.T) {
	agent, _, now := certAgent(t)
	n := &notes{}
	agent.certificate(n)

	writeReal := func(notAfter time.Time) {
		key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		tmpl := &x509.Certificate{SerialNumber: serial(), DNSNames: []string{"smith.zoomriya.app"},
			NotBefore: now.Add(-90 * 24 * time.Hour), NotAfter: notAfter}
		der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
		keyDER, _ := x509.MarshalECPrivateKey(key)
		data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		data = append(data, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})...)
		os.WriteFile(agent.path(tlsCerts+"/connect.pem"), data, 0o640)
	}

	writeReal(now.Add(5 * 24 * time.Hour))
	if err := agent.certificate(n); err != nil {
		t.Fatalf("a warning isn't a failure: %v", err)
	}
	agent.certificate(n)
	if len(n.list) != 1 || n.list[0].kind != "secure" || n.list[0].level != "warning" || !strings.Contains(n.list[0].message, "smith.zoomriya.app") {
		t.Fatalf("told once: %+v", n.list)
	}

	*now = now.Add(6 * 24 * time.Hour)
	if err := agent.certificate(n); err == nil {
		t.Fatal("a certificate that ran out fails the job")
	}
	if last := n.list[len(n.list)-1]; last.level != "error" || !strings.Contains(last.message, "ran out") {
		t.Fatalf("%+v", last)
	}

	writeReal(now.Add(90 * 24 * time.Hour))
	n.list = nil
	if err := agent.certificate(n); err != nil || len(n.list) != 1 || n.list[0].level != "info" {
		t.Fatalf("fixed: %v %+v", err, n.list)
	}

	// A broken bundle.
	os.WriteFile(agent.path(tlsCerts+"/connect.pem"), []byte("not a certificate"), 0o640)
	n.list = nil
	if err := agent.certificate(n); err == nil || len(n.list) != 1 || !strings.Contains(n.list[0].message, "connect.pem is broken") {
		t.Fatalf("broken: %v %+v", err, n.list)
	}
}

func TestCertificateNoName(t *testing.T) {
	agent, _, _ := certAgent(t)
	os.Remove(agent.path("etc/hostname"))
	n := &notes{}
	if err := agent.certificate(n); err == nil || len(n.list) != 1 || n.list[0].level != "error" {
		t.Fatalf("reported: %v %+v", err, n.list)
	}
}
