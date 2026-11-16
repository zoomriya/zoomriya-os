package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Fetcher asks the weather services, and only them: HTTPS with the
// certificate checked, a short timeout, a size limit, and our User-Agent, as
// NWS and MET Norway ask (weather.md §5, §6). Nothing it gets is ever run.
type Fetcher struct {
	Client *http.Client
	// Allowed: the hosts it may ask (the declared connection's); a URL a
	// service gives (NWS's forecast link) on any other host is refused.
	Allowed map[string]bool
	Agent   string
	// Limit: the largest answer read, in bytes.
	Limit int64
}

// Answer is one response: its body, and the headers the services' rules use.
type Answer struct {
	Status       int
	Body         []byte
	Expires      time.Time
	LastModified string
}

var errTooLarge = errors.New("the answer is too large")

// Get fetches rawURL. modifiedSince, when set, is sent as If-Modified-Since
// (MET Norway asks for it); a 304 comes back as an Answer with no body.
func (f *Fetcher) Get(ctx context.Context, rawURL, accept, modifiedSince string) (Answer, error) {
	u, err := url.Parse(rawURL)
	if err != nil || !f.Allowed[u.Host] {
		return Answer{}, fmt.Errorf("not a weather service's address")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Answer{}, err
	}
	req.Header.Set("User-Agent", f.Agent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if modifiedSince != "" {
		req.Header.Set("If-Modified-Since", modifiedSince)
	}
	resp, err := f.Client.Do(req)
	if err != nil {
		return Answer{}, fmt.Errorf("the weather service didn't answer")
	}
	defer resp.Body.Close()
	a := Answer{Status: resp.StatusCode, LastModified: resp.Header.Get("Last-Modified")}
	if t, err := http.ParseTime(resp.Header.Get("Expires")); err == nil {
		a.Expires = t
	}
	if resp.StatusCode == http.StatusNotModified {
		return a, nil
	}
	if resp.StatusCode != http.StatusOK {
		return a, fmt.Errorf("the weather service answered %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, f.Limit+1))
	if err != nil {
		return a, fmt.Errorf("the weather service's answer was cut short")
	}
	if int64(len(body)) > f.Limit {
		return a, errTooLarge
	}
	a.Body = body
	return a, nil
}

// getJSON fetches and reads JSON into v.
func (f *Fetcher) getJSON(ctx context.Context, rawURL, accept string, v any) (Answer, error) {
	a, err := f.Get(ctx, rawURL, accept, "")
	if err != nil {
		return a, err
	}
	if err := json.Unmarshal(a.Body, v); err != nil {
		return a, fmt.Errorf("the weather service's answer didn't read")
	}
	return a, nil
}
