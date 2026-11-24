package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestTurnsGoByPriorityThenOrder(t *testing.T) {
	turns := NewTurns()
	turns.IdleBeforeOvernight = 0
	release, _ := turns.Take(context.Background(), Interactive)

	var mu sync.Mutex
	var order []string
	var wg sync.WaitGroup
	ask := func(name, priority string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			done, err := turns.Take(context.Background(), priority)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
			done()
		}()
		time.Sleep(20 * time.Millisecond) // asked in this order
	}
	ask("overnight", Overnight)
	ask("background 1", Background)
	ask("member", Interactive)
	ask("background 2", Background)
	release()
	wg.Wait()
	want := []string{"member", "background 1", "background 2", "overnight"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("went in %v, want %v", order, want)
		}
	}
}

func TestOvernightWaitsForAQuietModel(t *testing.T) {
	turns := NewTurns()
	turns.IdleBeforeOvernight = 150 * time.Millisecond
	release, _ := turns.Take(context.Background(), Interactive)
	release()
	start := time.Now()
	done, err := turns.Take(context.Background(), Overnight)
	if err != nil {
		t.Fatal(err)
	}
	done()
	if waited := time.Since(start); waited < 100*time.Millisecond {
		t.Errorf("an overnight job went after %v, before the model was quiet", waited)
	}
}

func TestGivingUpLeavesTheQueue(t *testing.T) {
	turns := NewTurns()
	release, _ := turns.Take(context.Background(), Interactive)
	ctx, cancel := context.WithCancel(context.Background())
	gaveUp := make(chan error)
	go func() {
		_, err := turns.Take(ctx, Interactive)
		gaveUp <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := <-gaveUp; err == nil {
		t.Fatal("a cancelled wait took the model")
	}
	release()
	next, err := turns.Take(context.Background(), Background)
	if err != nil {
		t.Fatal(err)
	}
	next()
	if _, err := turns.Take(context.Background(), "urgent"); err == nil {
		t.Error("an unknown priority was accepted")
	}
}
