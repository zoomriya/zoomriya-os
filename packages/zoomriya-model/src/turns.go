// One answer at a time: the model is single-threaded on the box's CPU, so
// everyone waits their turn, most urgent first. A member waiting on an
// answer goes first, then an app's own work (Photos' descriptions, Myira's
// Memory), then overnight jobs, which also wait until no one has used the
// model for a minute. Priority only decides who goes next: an answer under
// way is never cut short.
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	Interactive = "interactive"
	Background  = "background"
	Overnight   = "overnight"
)

var priorities = []string{Interactive, Background, Overnight}

func rank(priority string) (int, error) {
	for i, p := range priorities {
		if p == priority {
			return i, nil
		}
	}
	return 0, fmt.Errorf("unknown priority: %s", priority)
}

type waiter struct{ rank, ticket int }

// Turns hands out the model: release, err := turns.Take(ctx, Background).
type Turns struct {
	// IdleBeforeOvernight: how long the model must have been quiet.
	IdleBeforeOvernight time.Duration
	Now                 func() time.Time

	mu       sync.Mutex
	busy     bool
	waiting  []waiter
	tickets  int
	lastUsed time.Time
	// changed is closed (and replaced) whenever someone may go.
	changed chan struct{}
}

func NewTurns() *Turns {
	return &Turns{IdleBeforeOvernight: time.Minute, Now: time.Now, changed: make(chan struct{})}
}

func (t *Turns) wake() {
	close(t.changed)
	t.changed = make(chan struct{})
}

// mayGo: zero when w may take the model now; else how long to wait, or -1
// to wait until woken.
func (t *Turns) mayGo(w waiter) time.Duration {
	if t.busy {
		return -1
	}
	for _, other := range t.waiting {
		if other.rank < w.rank || (other.rank == w.rank && other.ticket < w.ticket) {
			return -1
		}
	}
	if w.rank < 2 || t.lastUsed.IsZero() {
		return 0
	}
	quiet := t.Now().Sub(t.lastUsed)
	if quiet >= t.IdleBeforeOvernight {
		return 0
	}
	return t.IdleBeforeOvernight - quiet
}

func (t *Turns) remove(w waiter) {
	for i, other := range t.waiting {
		if other == w {
			t.waiting = append(t.waiting[:i], t.waiting[i+1:]...)
			return
		}
	}
}

// Take waits for the model; release gives it back. A cancelled ctx (the
// asker went away) leaves the queue, so the next may go.
func (t *Turns) Take(ctx context.Context, priority string) (release func(), err error) {
	r, err := rank(priority)
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	t.tickets++
	w := waiter{r, t.tickets}
	t.waiting = append(t.waiting, w)
	for {
		wait := t.mayGo(w)
		if wait == 0 {
			break
		}
		changed := t.changed
		t.mu.Unlock()
		var timer <-chan time.Time
		if wait > 0 {
			timer = time.After(wait)
		}
		select {
		case <-ctx.Done():
			t.mu.Lock()
			t.remove(w)
			t.wake()
			t.mu.Unlock()
			return nil, ctx.Err()
		case <-changed:
		case <-timer:
		}
		t.mu.Lock()
	}
	t.remove(w)
	t.busy = true
	t.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			t.mu.Lock()
			t.busy = false
			if r < 2 {
				t.lastUsed = t.Now()
			}
			t.wake()
			t.mu.Unlock()
		})
	}, nil
}

// Busy: an answer is under way.
func (t *Turns) Busy() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.busy
}
