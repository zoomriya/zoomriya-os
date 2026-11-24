// The box's certificates (design §8.1), as the gateway serves them: every
// bundle in certs/ (certificate, chain and key in one PEM file), picked by
// the name the browser asks for. ZOOMRIYA Connect's real certificate answers
// <name>.zoomriya.app; the box's own answers <name>.local and the box's home
// addresses. The agent and connectd renew them; they're read again when the
// folder changes.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Certificates struct {
	dir   string
	mu    sync.RWMutex
	certs []*tls.Certificate
	// What the folder looked like when it was read: each file's name, size
	// and time.
	seen string
	// Names, from every certificate: the box's own names, which a request's
	// Host must be one of.
	names map[string]bool
}

func NewCertificates(dir string) *Certificates {
	c := &Certificates{dir: dir}
	c.Load()
	return c
}

// Watch reads the folder again every 30 seconds when it changed.
func (c *Certificates) Watch(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if c.state() != c.current() {
				c.Load()
			}
		}
	}
}

func (c *Certificates) current() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.seen
}

func (c *Certificates) state() string {
	files, _ := filepath.Glob(filepath.Join(c.dir, "*.pem"))
	sort.Strings(files)
	var b strings.Builder
	for _, f := range files {
		if fi, err := os.Stat(f); err == nil {
			b.WriteString(f + "|" + fi.ModTime().String() + "|" + strconv.FormatInt(fi.Size(), 10) + "\n")
		}
	}
	return b.String()
}

// Load reads every bundle; one it can't read is left out and logged.
func (c *Certificates) Load() {
	state := c.state()
	files, _ := filepath.Glob(filepath.Join(c.dir, "*.pem"))
	sort.Strings(files)
	var certs []*tls.Certificate
	names := map[string]bool{}
	for _, f := range files {
		cert, err := loadBundle(f)
		if err != nil {
			log.Printf("box-desktopd: %s left out: %v", filepath.Base(f), err)
			continue
		}
		// The box's own (box.pem) last: it answers whatever no other one does.
		if filepath.Base(f) == "box.pem" {
			certs = append(certs, cert)
		} else {
			certs = append([]*tls.Certificate{cert}, certs...)
		}
		for _, n := range cert.Leaf.DNSNames {
			names[strings.ToLower(n)] = true
		}
		for _, ip := range cert.Leaf.IPAddresses {
			names[ip.String()] = true
		}
	}
	c.mu.Lock()
	c.certs, c.names, c.seen = certs, names, state
	c.mu.Unlock()
}

func loadBundle(path string) (*tls.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var certPEM, keyPEM []byte
	for rest := data; ; {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			certPEM = append(certPEM, pem.EncodeToMemory(block)...)
		} else if strings.HasSuffix(block.Type, "PRIVATE KEY") {
			keyPEM = pem.EncodeToMemory(block)
		}
	}
	if certPEM == nil || keyPEM == nil {
		return nil, errors.New("no certificate or no key")
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	if cert.Leaf == nil {
		if cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0]); err != nil {
			return nil, err
		}
	}
	if time.Now().After(cert.Leaf.NotAfter) {
		return nil, errors.New("run out on " + cert.Leaf.NotAfter.Format(time.DateOnly))
	}
	return &cert, nil
}

// Get picks the certificate for the name asked for: the first that's valid
// for it, else the box's own.
func (c *Certificates) Get(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.certs) == 0 {
		return nil, errors.New("no certificate yet")
	}
	name := strings.ToLower(strings.TrimSuffix(hello.ServerName, "."))
	if name == "" {
		// An address typed in: the certificate that names it.
		if host, _, err := net.SplitHostPort(hello.Conn.LocalAddr().String()); err == nil {
			name = host
		}
	}
	for _, cert := range c.certs {
		if cert.Leaf.VerifyHostname(name) == nil {
			return cert, nil
		}
	}
	return c.certs[len(c.certs)-1], nil
}

// Names says whether host is one of the box's own names or addresses, from
// its certificates.
func (c *Certificates) Names(host string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if c.names[host] {
		return true
	}
	for _, cert := range c.certs {
		if cert.Leaf.VerifyHostname(host) == nil {
			return true
		}
	}
	return false
}
