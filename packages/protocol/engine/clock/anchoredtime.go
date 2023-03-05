package clock

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/runtime/event"
)

// AnchoredTime is a time value that advances with the system clock but that is anchored to a specific point in time.
type AnchoredTime struct {
	OnAnchorUpdated *event.Event1[time.Time]

	anchor     time.Time
	updateTime time.Time
	mutex      sync.RWMutex
}

func NewAnchoredTime() *AnchoredTime {
	return &AnchoredTime{
		OnAnchorUpdated: event.New1[time.Time](),
	}
}

func (c *AnchoredTime) Get() time.Time {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.anchor
}

func (c *AnchoredTime) Now() time.Time {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.anchor.Add(time.Since(c.updateTime))
}

func (c *AnchoredTime) Set(newTime time.Time) (updated bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if newTime.Before(c.anchor) {
		return false
	}

	c.updateTime = time.Now()
	c.anchor = newTime

	c.OnAnchorUpdated.Trigger(c.anchor)

	return true
}

// Advance advances the time monotonically if the given time is after the current time.
func (c *AnchoredTime) Advance(newTime time.Time) (updated bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if newTime.Before(c.anchor) {
		return false
	}

	c.updateTime = c.advancedUpdateTime(newTime)
	c.anchor = newTime

	c.OnAnchorUpdated.Trigger(c.anchor)

	return true
}

// advancedUpdateTime determines the new update time that is in sync with the monotonic clock.
func (c *AnchoredTime) advancedUpdateTime(newTime time.Time) time.Time {
	diff := time.Since(c.updateTime)

	// if the new time lags behind the monotonic time, we adjust the time to prevent the clock from going backwards.
	if lag := newTime.Sub(c.anchor.Add(diff)); lag < 0 {
		diff += lag
	}

	return c.updateTime.Add(diff)
}
