// Package hlc implements a hybrid logical clock (Kulkarni et al., 2014).
//
// A timestamp is packed into an int64: the upper 48 bits hold wall-clock
// milliseconds and the lower 16 bits hold a logical counter. Packed values
// compare with ordinary integer ordering, so they can be stored in a BIGINT
// column and sorted directly.
package hlc

import (
	"sync"
	"time"
)

const logicalBits = 16
const logicalMask = 1<<logicalBits - 1

// Timestamp is a packed HLC value.
type Timestamp int64

// New packs wall-clock milliseconds and a logical counter.
func New(wallMillis int64, logical uint16) Timestamp {
	return Timestamp(wallMillis<<logicalBits | int64(logical))
}

// WallMillis returns the physical component in Unix milliseconds.
func (t Timestamp) WallMillis() int64 { return int64(t) >> logicalBits }

// Logical returns the logical counter.
func (t Timestamp) Logical() uint16 { return uint16(int64(t) & logicalMask) }

// Time returns the physical component as a time.Time.
func (t Timestamp) Time() time.Time { return time.UnixMilli(t.WallMillis()) }

// Clock issues monotonically increasing timestamps. It is safe for
// concurrent use.
type Clock struct {
	mu   sync.Mutex
	last Timestamp
	now  func() int64
}

// NewClock returns a clock driven by the system wall clock.
func NewClock() *Clock {
	return &Clock{now: func() int64 { return time.Now().UnixMilli() }}
}

// newClockWithSource is used by tests to control physical time.
func newClockWithSource(now func() int64) *Clock { return &Clock{now: now} }

// Now returns a timestamp for a local or send event.
func (c *Clock) Now() Timestamp {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.last = c.advance(c.last, 0)
	return c.last
}

// Update merges a timestamp received from another node and returns a
// timestamp for the receive event, which is greater than both.
func (c *Clock) Update(remote Timestamp) Timestamp {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.last = c.advance(c.last, remote)
	return c.last
}

func (c *Clock) advance(last, remote Timestamp) Timestamp {
	pt := c.now()
	wall := max(last.WallMillis(), remote.WallMillis(), pt)

	var logical int64
	switch {
	case wall == last.WallMillis() && wall == remote.WallMillis():
		logical = int64(max(last.Logical(), remote.Logical())) + 1
	case wall == last.WallMillis():
		logical = int64(last.Logical()) + 1
	case wall == remote.WallMillis():
		logical = int64(remote.Logical()) + 1
	}
	if logical > logicalMask {
		// Counter overflow: borrow a millisecond rather than wrap.
		wall++
		logical = 0
	}
	return New(wall, uint16(logical))
}
