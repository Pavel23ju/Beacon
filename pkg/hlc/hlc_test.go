package hlc

import (
	"sync"
	"testing"
)

func TestPackRoundTrip(t *testing.T) {
	ts := New(1_700_000_000_123, 42)
	if ts.WallMillis() != 1_700_000_000_123 || ts.Logical() != 42 {
		t.Fatalf("got wall=%d logical=%d", ts.WallMillis(), ts.Logical())
	}
}

func TestNowMonotonicWhenWallClockStalls(t *testing.T) {
	c := newClockWithSource(func() int64 { return 1000 })
	a, b := c.Now(), c.Now()
	if b <= a {
		t.Fatalf("expected %d > %d", b, a)
	}
	if b.WallMillis() != 1000 || b.Logical() != 1 {
		t.Fatalf("got wall=%d logical=%d", b.WallMillis(), b.Logical())
	}
}

func TestNowMonotonicWhenWallClockGoesBackwards(t *testing.T) {
	pt := int64(2000)
	c := newClockWithSource(func() int64 { return pt })
	a := c.Now()
	pt = 1500
	if b := c.Now(); b <= a {
		t.Fatalf("expected %d > %d", b, a)
	}
}

func TestUpdateJumpsPastRemote(t *testing.T) {
	c := newClockWithSource(func() int64 { return 1000 })
	remote := New(5000, 7)
	got := c.Update(remote)
	if got <= remote {
		t.Fatalf("expected %d > remote %d", got, remote)
	}
	if got.WallMillis() != 5000 || got.Logical() != 8 {
		t.Fatalf("got wall=%d logical=%d", got.WallMillis(), got.Logical())
	}
	if next := c.Now(); next <= got {
		t.Fatalf("expected Now after Update to advance, got %d <= %d", next, got)
	}
}

func TestLogicalOverflowBorrowsMillisecond(t *testing.T) {
	c := newClockWithSource(func() int64 { return 1000 })
	c.last = New(1000, logicalMask)
	got := c.Now()
	if got.WallMillis() != 1001 || got.Logical() != 0 {
		t.Fatalf("got wall=%d logical=%d", got.WallMillis(), got.Logical())
	}
}

func TestConcurrentNowIsUnique(t *testing.T) {
	c := NewClock()
	const n = 1000
	seen := make(chan Timestamp, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() { defer wg.Done(); seen <- c.Now() }()
	}
	wg.Wait()
	close(seen)
	uniq := map[Timestamp]bool{}
	for ts := range seen {
		if uniq[ts] {
			t.Fatalf("duplicate timestamp %d", ts)
		}
		uniq[ts] = true
	}
}
