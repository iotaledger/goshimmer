package pow

import (
	"sort"
	"sync"
	"time"
)

var (
	// ApowWindow defines the time window of the apow (should be greater or equal to the timestamp quality precision).
	ApowWindow int
	// BaseDifficulty defines the base difficulty of the proof-of-work.
	BaseDifficulty int
	// AdaptiveRate defines the rate at which increase the proof-of-work difficulty.
	AdaptiveRate float64
)

// MessageAge defines the pair messageID (as string) and its issuance timestamp.
type MessageAge struct {
	ID        string
	Timestamp time.Time
}

type messagesWindow []MessageAge

func (m messagesWindow) Len() int { return len(m) }
func (m messagesWindow) Less(i, j int) bool {
	if m[i].Timestamp.Before(m[j].Timestamp) ||
		(m[i].Timestamp.Equal(m[j].Timestamp) && m[i].ID < m[j].ID) { // lexicographical check
		return true
	}
	return false
}
func (m messagesWindow) Swap(i, j int) { m[i], m[j] = m[j], m[i] }

// MessagesWindow is a concurrent-safe slice of MessageAge.
type MessagesWindow struct {
	internalSlice messagesWindow
	sync.RWMutex
}

// getDifficulty returns the adaptive pow difficulty.
func (m *MessagesWindow) getDifficulty(t time.Time) int {
	return BaseDifficulty + int(AdaptiveRate*float64(m.recentMessages(t)))
}

// head returns the index of the oldest message within the time window t-ApowWindow.
// If no messages are within the given interval, it returns -1.
func (m *MessagesWindow) head(t time.Time) int {
	sort.Sort(m.internalSlice)

	for i, v := range m.internalSlice {
		if v.Timestamp.Add(time.Duration(ApowWindow) * time.Second).After(t) {
			return i
		}
	}
	return -1
}

// recentMessages returns the number of the recent messsages within the time window t-ApowWindow.
func (m *MessagesWindow) recentMessages(t time.Time) int {
	head := m.head(t)
	switch head {
	case -1:
		return 0
	default:
		return len(m.internalSlice[head:])
	}
}

func (m *MessagesWindow) update(msg MessageAge) {
	head := m.head(msg.Timestamp)
	switch head {
	case -1:
		m.internalSlice = messagesWindow{msg}
	default:
		m.internalSlice = append(m.internalSlice[head:], msg)
	}
}

// Add adds a new messageAge and eventually cleans the messagesWindow from older elements.
func (m *MessagesWindow) Add(messageAge MessageAge) {
	m.Lock()
	defer m.Unlock()

	m.update(messageAge)
}

// AdaptiveDifficulty returns the adaptive proof-of-work difficulty.
func (m *MessagesWindow) AdaptiveDifficulty(t time.Time) int {
	m.RLock()
	defer m.RUnlock()

	return m.getDifficulty(t)
}

// CheckDifficulty atomically checks the correctness of the pow difficult and updates the messagesWindow.
func (m *MessagesWindow) CheckDifficulty(msg MessageAge, d int) bool {
	m.Lock()
	defer m.Unlock()

	if m.getDifficulty(msg.Timestamp) <= d {
		m.update(msg)
		return true
	}

	return false
}
