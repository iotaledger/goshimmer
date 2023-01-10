package clock

import (
	"sync"
	"time"
)

type timeUpdate struct {
	Time    time.Time
	Updated time.Time
	sync.RWMutex
}
