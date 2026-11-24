// Box Desktop's own sign-ins (box-desktop.md §7, §8): one per browser, kept
// in memory only, so a restart of box-desktopd signs everyone out. The
// browser holds a random cookie; ZOOMRIYA ID's token and session cookie stay
// here. A sign-in ends after 12 hours, or 30 minutes with no page open.
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"sync"
	"time"
)

const (
	sessionLifetime = 12 * time.Hour
	sessionIdle     = 30 * time.Minute
	// A second-step ticket's username is kept this long, for the journal.
	pendingLifetime = 10 * time.Minute
	// After the password was asked for a restore or a reset, the restart
	// that follows isn't asked again (the native desktop's restart_now).
	restartGrace = 2 * time.Minute
)

type Session struct {
	ID       string
	Token    string // sent with every change (X-Box-Desktop-Token)
	Listener Listener
	Remote   string
	Created  time.Time

	mu        sync.Mutex
	login     *Login
	seen      time.Time
	streams   int // pages open, through the event stream
	restartOK time.Time
	// Done is closed when the sign-in ends: the page's event stream says so.
	Done chan struct{}
	// locked: the screen was locked in this browser; only the password
	// (or signing out) goes on from here.
	locked bool
}

func (s *Session) Locked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.locked
}

func (s *Session) SetLocked(on bool) {
	s.mu.Lock()
	s.locked = on
	s.mu.Unlock()
}

func (s *Session) Login() *Login {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.login
}

func (s *Session) touch(now time.Time) {
	s.mu.Lock()
	s.seen = now
	s.mu.Unlock()
}

type Sessions struct {
	mu      sync.Mutex
	byID    map[string]*Session
	pending map[string]pendingSignIn
	Now     func() time.Time
	// Ended is told about each sign-in that ran out (to sign out of ZOOMRIYA ID).
	Ended func(*Session)
}

type pendingSignIn struct {
	username string
	until    time.Time
}

func NewSessions() *Sessions {
	return &Sessions{byID: map[string]*Session{}, pending: map[string]pendingSignIn{}, Now: time.Now}
}

func random() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Start keeps a new sign-in.
func (ss *Sessions) Start(login *Login, l Listener, remote string) *Session {
	now := ss.Now()
	s := &Session{ID: random(), Token: random(), Listener: l, Remote: remote, Created: now, login: login, seen: now,
		Done: make(chan struct{})}
	ss.mu.Lock()
	ss.byID[s.ID] = s
	ss.mu.Unlock()
	return s
}

// Find gives the sign-in for a cookie, if it's still on and from the same
// side (the home network's or the box's screen's).
func (ss *Sessions) Find(id string, l Listener) *Session {
	if id == "" {
		return nil
	}
	ss.mu.Lock()
	s := ss.byID[id]
	ss.mu.Unlock()
	if s == nil || s.Listener != l {
		return nil
	}
	if ss.expired(s, ss.Now()) {
		ss.End(s)
		return nil
	}
	s.touch(ss.Now())
	return s
}

func (ss *Sessions) expired(s *Session, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now.Sub(s.Created) > sessionLifetime {
		return true
	}
	return s.streams == 0 && now.Sub(s.seen) > sessionIdle
}

// End forgets a sign-in.
func (ss *Sessions) End(s *Session) {
	ss.mu.Lock()
	_, there := ss.byID[s.ID]
	delete(ss.byID, s.ID)
	ss.mu.Unlock()
	if !there {
		return
	}
	close(s.Done)
	if ss.Ended != nil {
		ss.Ended(s)
	}
}

// EndMember signs a member out everywhere here (their account turned off,
// or no longer an admin).
func (ss *Sessions) EndMember(memberID int) {
	ss.mu.Lock()
	var gone []*Session
	for _, s := range ss.byID {
		if l := s.Login(); l != nil && l.Member.ID == memberID {
			gone = append(gone, s)
		}
	}
	ss.mu.Unlock()
	for _, s := range gone {
		ss.End(s)
	}
}

// Sweep ends the sign-ins that ran out, every minute.
func (ss *Sessions) Sweep(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			now := ss.Now()
			ss.mu.Lock()
			var gone []*Session
			for _, s := range ss.byID {
				if ss.expired(s, now) {
					gone = append(gone, s)
				}
			}
			for t, p := range ss.pending {
				if now.After(p.until) {
					delete(ss.pending, t)
				}
			}
			ss.mu.Unlock()
			for _, s := range gone {
				ss.End(s)
			}
		}
	}
}

// Pending keeps who a second-step ticket is for, for the journal.
func (ss *Sessions) Pending(ticket, username string) {
	if ticket == "" {
		return
	}
	ss.mu.Lock()
	ss.pending[ticket] = pendingSignIn{username, ss.Now().Add(pendingLifetime)}
	ss.mu.Unlock()
}

func (ss *Sessions) PendingUser(ticket string) string {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.pending[ticket].username
}

// Stream counts a page's event stream open (+1) or closed (-1).
func (s *Session) Stream(delta int) {
	s.mu.Lock()
	s.streams += delta
	s.seen = time.Now()
	s.mu.Unlock()
}

func (s *Session) TokenMatches(t string) bool {
	return t != "" && subtle.ConstantTimeCompare([]byte(t), []byte(s.Token)) == 1
}

func (s *Session) AllowRestart(now time.Time) {
	s.mu.Lock()
	s.restartOK = now.Add(restartGrace)
	s.mu.Unlock()
}

// RestartAllowed uses up the restart that follows a restore or a reset.
func (s *Session) RestartAllowed(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ok := now.Before(s.restartOK)
	s.restartOK = time.Time{}
	return ok
}

// AtScreen: this member is signed in at the box's own screen.
func (ss *Sessions) AtScreen(memberID int) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for _, s := range ss.byID {
		if l := s.Login(); l != nil && s.Listener == ListenerScreen && l.Member.ID == memberID {
			return true
		}
	}
	return false
}

func (ss *Sessions) Count() int {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return len(ss.byID)
}
