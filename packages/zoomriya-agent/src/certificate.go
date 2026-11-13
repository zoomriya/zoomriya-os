// The box's certificates (design §8.1): `zoomriya-agent certificate`, at each
// start and every five minutes, so the apps are reached over HTTPS at home with
// nothing for anyone to set up.
//
//   - The box makes its own certificate authority once, limited to names
//     ending in .local and the home network's addresses, so even if it were
//     copied it couldn't vouch for any site on the internet.
//   - With it, the box's own certificate for <name>.local and the box's
//     current home addresses, remade when they change and a month before it
//     runs out. The phone and desktop apps pin the authority when they pair.
//   - Every certificate in certs/ is watched, the box's own and the real one
//     ZOOMRIYA Connect's connectd requests and renews for <name>.zoomriya.app:
//     a certificate running out, run out or broken goes to the bell, once, and
//     so does the news when it's fixed.
//
// The gateway loads every bundle in certs/ (certificate, chain and key, one
// file each) and picks one by the name asked for; a path unit restarts it when
// one changes. Group zoomriya-tls may read them; only root reads the
// authority's key.
package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	tlsDir    = "etc/zoomriya/tls"
	tlsCAKey  = tlsDir + "/ca.key"
	tlsCACert = tlsDir + "/ca.crt" // public: what a family member's device trusts
	tlsCerts  = tlsDir + "/certs"
	tlsOwn    = tlsCerts + "/box.pem"
	tlsGroup  = "zoomriya-tls"
	// Where the gateway serves the authority for a device to trust.
	trustPath   = "/zoomriya-box.crt"
	tlsState    = "var/lib/zoomriya-agent/certificate.json"
	ownLifetime = 397 * 24 * time.Hour // what browsers accept for a server certificate
	ownRenew    = 30 * 24 * time.Hour
	caLifetime  = 20 * 365 * 24 * time.Hour
	// A certificate the box doesn't renew itself (connectd's) is reported
	// when it has this little left.
	tlsWarnLeft = 14 * 24 * time.Hour
)

// The home network's addresses: all the authority may vouch for, with .local.
var homeRanges = func() []*net.IPNet {
	var list []*net.IPNet
	for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"} {
		_, n, _ := net.ParseCIDR(cidr)
		list = append(list, n)
	}
	return list
}()

func atHome(ip net.IP) bool {
	return slices.ContainsFunc(homeRanges, func(n *net.IPNet) bool { return n.Contains(ip) })
}

// What the job reported last time, so each problem is told once.
type certificateState struct {
	Problems []string `json:"problems"`
}

type tlsProblem struct{ level, message string }

func certificateJob() int {
	if err := NewAgent().certificate(NewJournal()); err != nil {
		return 1
	}
	return 0
}

// certificate makes or renews the box's own certificate, then looks at every
// certificate the gateway serves. An error means the bell has been told.
func (a *Agent) certificate(n Notifier) error {
	var problems []tlsProblem
	if p := a.ownCertificate(); p != nil {
		problems = append(problems, *p)
	}
	problems = append(problems, a.watchCertificates()...)

	var last certificateState
	if data, err := os.ReadFile(a.path(tlsState)); err == nil {
		_ = json.Unmarshal(data, &last)
	}
	now := certificateState{Problems: []string{}}
	failed := false
	for _, p := range problems {
		now.Problems = append(now.Problems, p.message)
		if !slices.Contains(last.Problems, p.message) {
			n.Notify("secure", p.level, p.message)
		}
		failed = failed || p.level == "error"
	}
	if len(last.Problems) > 0 && len(now.Problems) == 0 {
		n.Notify("secure", "info", "Secure connections at home work again.")
	}
	data, _ := json.Marshal(now)
	if err := os.MkdirAll(filepath.Dir(a.path(tlsState)), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(a.path(tlsState), data, 0o600); err != nil {
		return err
	}
	if failed {
		return errors.New(strings.Join(now.Problems, " "))
	}
	return nil
}

// ownCertificate makes the authority and the box's certificate when they're
// missing, and the certificate again when its names or addresses changed or
// it's a month from running out.
func (a *Agent) ownCertificate() *tlsProblem {
	fail := func(what string, err error) *tlsProblem {
		return &tlsProblem{"error", fmt.Sprintf("The box couldn't %s, so the apps may not open securely at home (%v). Restart the box; if this stays, open Support Center.", what, err)}
	}
	name := a.boxAddress()
	if name == "" {
		return fail("make its certificate", errors.New("the box has no name yet"))
	}
	ips, err := a.homeAddresses()
	if err != nil {
		return fail("read its home addresses", err)
	}
	gid, err := a.tlsGID()
	if err != nil {
		return fail("make its certificate", err)
	}

	ca, caKey, err := a.loadCA()
	if err != nil {
		if ca, caKey, err = a.makeCA(name, gid); err != nil {
			return fail("make its certificate authority", err)
		}
	}
	if a.ownCurrent(ca, name, ips) {
		return nil
	}
	if err := a.makeOwn(ca, caKey, name, ips, gid); err != nil {
		return fail("renew its certificate", err)
	}
	return nil
}

// homeAddresses are the box's addresses on the home network, sorted.
func (a *Agent) homeAddresses() ([]net.IP, error) {
	out, err := a.Run("ip", "-json", "address", "show")
	if err != nil {
		return nil, err
	}
	var addrs []ipAddr
	if err := json.Unmarshal(out, &addrs); err != nil {
		return nil, fmt.Errorf("ip's addresses: %w", err)
	}
	var ips []net.IP
	for _, ad := range addrs {
		if a.linkKind(ad) == "" {
			continue
		}
		for _, info := range ad.AddrInfo {
			if ip := net.ParseIP(info.Local); ip != nil && info.Scope == "global" && atHome(ip) {
				ips = append(ips, ip)
			}
		}
	}
	slices.SortFunc(ips, func(x, y net.IP) int { return bytes.Compare(x.To16(), y.To16()) })
	return ips, nil
}

// tlsGID is the group that may read the certificates; in tests, the test's own.
func (a *Agent) tlsGID() (int, error) {
	if a.Root != "/" {
		return os.Getgid(), nil
	}
	g, err := user.LookupGroup(tlsGroup)
	if err != nil {
		return 0, fmt.Errorf("no group %s", tlsGroup)
	}
	return strconv.Atoi(g.Gid)
}

func (a *Agent) loadCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	certPEM, err := os.ReadFile(a.path(tlsCACert))
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := os.ReadFile(a.path(tlsCAKey))
	if err != nil {
		return nil, nil, err
	}
	cert, err := firstCertificate(certPEM)
	if err != nil {
		return nil, nil, err
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, nil, errors.New("no key")
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, err
	}
	if !key.PublicKey.Equal(cert.PublicKey) || a.Now().After(cert.NotAfter) {
		return nil, nil, errors.New("the authority doesn't match its key, or ran out")
	}
	return cert, key, nil
}

func (a *Agent) makeCA(name string, gid int) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	now := a.Now()
	template := &x509.Certificate{
		SerialNumber:                serial(),
		Subject:                     pkix.Name{CommonName: "ZOOMRIYA Box " + strings.TrimSuffix(name, ".local"), Organization: []string{"ZOOMRIYA Box"}},
		NotBefore:                   now.Add(-time.Hour),
		NotAfter:                    now.Add(caLifetime),
		KeyUsage:                    x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid:       true,
		IsCA:                        true,
		MaxPathLenZero:              true,
		PermittedDNSDomainsCritical: true,
		PermittedDNSDomains:         []string{"local"},
		PermittedIPRanges:           homeRanges,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	cert, _ := x509.ParseCertificate(der)
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	if err := a.tlsFolders(gid); err != nil {
		return nil, nil, err
	}
	if err := writeFileAtomic(a.path(tlsCAKey), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return nil, nil, err
	}
	if err := writeFileAtomic(a.path(tlsCACert), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

// ownCurrent: the box's certificate is from this authority, for exactly this
// name and these addresses, with more than a month left.
func (a *Agent) ownCurrent(ca *x509.Certificate, name string, ips []net.IP) bool {
	data, err := os.ReadFile(a.path(tlsOwn))
	if err != nil {
		return false
	}
	cert, err := firstCertificate(data)
	if err != nil || cert.CheckSignatureFrom(ca) != nil || cert.NotAfter.Sub(a.Now()) < ownRenew {
		return false
	}
	if !slices.Equal(cert.DNSNames, []string{name}) || len(cert.IPAddresses) != len(ips) {
		return false
	}
	for i, ip := range cert.IPAddresses {
		if !ip.Equal(ips[i]) {
			return false
		}
	}
	return true
}

func (a *Agent) makeOwn(ca *x509.Certificate, caKey *ecdsa.PrivateKey, name string, ips []net.IP, gid int) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	now := a.Now()
	template := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(ownLifetime),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{name},
		IPAddresses:  ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	var bundle bytes.Buffer
	pem.Encode(&bundle, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	pem.Encode(&bundle, &pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})
	pem.Encode(&bundle, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := a.tlsFolders(gid); err != nil {
		return err
	}
	return writeShared(a.path(tlsOwn), bundle.Bytes(), gid)
}

// tlsFolders: the folder only root and zoomriya-tls may enter.
func (a *Agent) tlsFolders(gid int) error {
	for _, dir := range []string{tlsDir, tlsCerts} {
		if err := os.MkdirAll(a.path(dir), 0o750); err != nil {
			return err
		}
		if err := os.Chown(a.path(dir), -1, gid); err != nil {
			return err
		}
		if err := os.Chmod(a.path(dir), 0o750); err != nil {
			return err
		}
	}
	return nil
}

// writeShared writes a file the group may read, whole or not at all. The
// temporary file is made beside certs/, not in it, so the gateway's path unit
// sees one change: the finished bundle arriving.
func writeShared(path string, data []byte, gid int) error {
	temp := filepath.Join(filepath.Dir(filepath.Dir(path)), "."+filepath.Base(path)+".new")
	if err := os.WriteFile(temp, data, 0o640); err != nil {
		return err
	}
	if err := os.Chown(temp, -1, gid); err != nil {
		return err
	}
	if err := os.Chmod(temp, 0o640); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// watchCertificates looks at every bundle the gateway serves.
func (a *Agent) watchCertificates() []tlsProblem {
	files, _ := filepath.Glob(filepath.Join(a.path(tlsCerts), "*.pem"))
	var problems []tlsProblem
	for _, file := range files {
		data, err := os.ReadFile(file)
		var cert *x509.Certificate
		if err == nil {
			cert, err = firstCertificate(data)
		}
		if err == nil && !hasKey(data, cert) {
			err = errors.New("its key is missing or doesn't match")
		}
		if err != nil {
			problems = append(problems, tlsProblem{"error", fmt.Sprintf("The certificate %s is broken (%v), so the apps may not open securely. Open Support Center.", filepath.Base(file), err)})
			continue
		}
		names := strings.Join(cert.DNSNames, ", ")
		left := cert.NotAfter.Sub(a.Now())
		switch {
		case left <= 0:
			problems = append(problems, tlsProblem{"error", fmt.Sprintf("The certificate for %s ran out on %s, so the apps won't open securely at that address. Open Support Center.", names, cert.NotAfter.Format("2 January 2006"))})
		case left < tlsWarnLeft:
			problems = append(problems, tlsProblem{"warning", fmt.Sprintf("The certificate for %s runs out on %s and hasn't been renewed yet.", names, cert.NotAfter.Format("2 January 2006"))})
		}
	}
	return problems
}

func firstCertificate(data []byte) (*x509.Certificate, error) {
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return nil, errors.New("no certificate")
		}
		if block.Type == "CERTIFICATE" {
			return x509.ParseCertificate(block.Bytes)
		}
	}
}

// hasKey: the bundle holds the private key of its first certificate.
func hasKey(data []byte, cert *x509.Certificate) bool {
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return false
		}
		var key any
		var err error
		switch block.Type {
		case "EC PRIVATE KEY":
			key, err = x509.ParseECPrivateKey(block.Bytes)
		case "PRIVATE KEY":
			key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		case "RSA PRIVATE KEY":
			key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		default:
			continue
		}
		if err != nil {
			return false
		}
		if k, ok := key.(crypto.Signer); ok {
			if p, ok := k.Public().(interface{ Equal(crypto.PublicKey) bool }); ok {
				return p.Equal(cert.PublicKey)
			}
		}
		return false
	}
}

// certificateStatus is for Control Panel › Security: where a device gets the
// box's authority to trust it, its fingerprint to compare, how long the box's
// own certificate lasts, and the problems the last look found.
func (a *Agent) certificateStatus() Response {
	status := Response{"fingerprint": nil, "trust_at": nil, "until": nil, "problems": []string{}}
	if data, err := os.ReadFile(a.path(tlsCACert)); err == nil {
		if ca, err := firstCertificate(data); err == nil {
			status["fingerprint"] = caFingerprint(ca)
		}
	}
	if name := a.boxAddress(); name != "" {
		status["trust_at"] = "http://" + name + trustPath
	}
	if data, err := os.ReadFile(a.path(tlsOwn)); err == nil {
		if cert, err := firstCertificate(data); err == nil {
			status["until"] = cert.NotAfter.UTC().Format(time.RFC3339)
		}
	}
	var last certificateState
	if data, err := os.ReadFile(a.path(tlsState)); err == nil && json.Unmarshal(data, &last) == nil && last.Problems != nil {
		status["problems"] = last.Problems
	}
	return status
}

// caFingerprint is what an admin compares with what a device shows.
func caFingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":")
}

func serial() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	return n
}
