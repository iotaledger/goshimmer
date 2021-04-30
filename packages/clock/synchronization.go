package clock

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/beevik/ntp"
)

// ErrNTPQueryFailed is returned if an NTP query failed.
var ErrNTPQueryFailed = errors.New("NTP query failed")

// difference between network time and node's local time.
var (
	offset      time.Duration
	offsetMutex sync.RWMutex
)

// FetchTimeOffset establishes the difference in local vs network time.
// This difference is stored in offset so that it can be used to adjust the local clock.
func FetchTimeOffset(host string) error {
	resp, err := ntp.Query(host)
	if err != nil {
		return errors.Errorf("NTP query error (%v): %w", err, ErrNTPQueryFailed)
	}
	offsetMutex.Lock()
	defer offsetMutex.Unlock()
	offset = resp.ClockOffset

	return nil
}

// SyncedTime gets the synchronized time (according to the network) of a node.
func SyncedTime() time.Time {
	offsetMutex.RLock()
	defer offsetMutex.RUnlock()

	return time.Now().Add(offset)
}

// Since returns the time elapsed since t.
// It is shorthand for clock.SyncedTime().Sub(t).
func Since(t time.Time) time.Duration {
	return SyncedTime().Sub(t)
}
