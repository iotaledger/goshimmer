package clock

import (
	"sync"
	"time"
)

// AnchoredTime is a time value that is anchored to a specific point in time but that advances with the real-time.
type AnchoredTime struct {
	anchor     time.Time
	updateTime time.Time
	mutex      sync.RWMutex
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

	if updated = newTime.Unix() > c.anchor.Unix(); updated {
		c.anchor = newTime
		c.updateTime = time.Now()
	}

	return
}
