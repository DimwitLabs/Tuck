package server

import (
	"strconv"
	"testing"
	"time"
)

func TestFullLimiterDropsStaleKeysBeforeRefusing(t *testing.T) {
	l := newLimiter(2, time.Minute)
	stale := time.Now().Add(-2 * time.Minute)
	for i := 0; i < maxKeys; i++ {
		l.hits[strconv.Itoa(i)] = []time.Time{stale}
	}
	if !l.allow("owner") {
		t.Fatal("stale keys should make room for a new one")
	}
	if len(l.hits) != 1 {
		t.Fatalf("stale keys should be gone, %d left", len(l.hits))
	}
	for i := 0; i < maxKeys; i++ {
		l.hits[strconv.Itoa(i)] = []time.Time{time.Now()}
	}
	if l.allow("newcomer") {
		t.Fatal("a limiter full of live keys refuses new ones")
	}
	if !l.allow("owner") {
		t.Fatal("a key that is already present keeps its allowance")
	}
}

func TestFullLimiterScansAtMostOncePerInterval(t *testing.T) {
	l := newLimiter(2, time.Minute)
	stale := time.Now().Add(-2 * time.Minute)
	fill := func() {
		for i := 0; i < maxKeys; i++ {
			l.hits[strconv.Itoa(i)] = []time.Time{stale}
		}
	}
	l.pruned = time.Now()
	fill()
	if l.allow("newcomer") {
		t.Fatal("a scan so soon after the last one should be skipped")
	}
	l.pruned = time.Now().Add(-pruneEvery)
	if !l.allow("newcomer") {
		t.Fatal("once the interval passes, stale keys make room again")
	}
}

func TestSweepForgetsFinishedWindows(t *testing.T) {
	l := newLimiter(1, 10*time.Millisecond)
	if !l.allow("one") || !l.allow("two") {
		t.Fatal("the first hit for each key should be allowed")
	}
	if l.allow("one") {
		t.Fatal("the second hit inside the window should be refused")
	}
	time.Sleep(15 * time.Millisecond)
	l.sweep()
	l.mu.Lock()
	left := len(l.hits)
	l.mu.Unlock()
	if left != 0 {
		t.Fatalf("a swept limiter should remember nothing, still holds %d keys", left)
	}
	if !l.allow("one") {
		t.Fatal("a key whose window passed should be allowed again")
	}
}
