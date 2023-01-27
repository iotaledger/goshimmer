package clock

import (
	"fmt"
	"sync"
	"time"
)

type timeUpdate struct {
	time    time.Time
	updated time.Time
	mutex   sync.RWMutex
}

func (c *timeUpdate) Time() time.Time {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.time
}

func (c *timeUpdate) RelativeTime() time.Time {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.time.Add(time.Since(c.updated))
}

// Update updates the given Time parameter if the given Time larger than the current Time.
func (c *timeUpdate) Update(now, newTime time.Time) (updated bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// the local wall clock should never be before the accepted Time unless we are eclipsed by malicious actors or our
	// own Time is clearly in the past
	if now.Before(newTime) {
		panic(fmt.Sprintf("tried to set Time is in the future. now: %s, newTime: %s", now.String(), newTime.String()))
	}

	if updated = newTime.Unix() > c.time.Unix(); updated {
		c.time = newTime
		c.updated = now
	}

	return
}
