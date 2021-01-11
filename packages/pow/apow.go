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

func (m MessageAge) less(j MessageAge) bool {
	if m.Timestamp.Before(j.Timestamp) ||
		(m.Timestamp.Equal(j.Timestamp) && m.ID < j.ID) { // lexicographical check
		return true
	}
	return false
}

type messagesWindow []MessageAge

func (m messagesWindow) Len() int           { return len(m) }
func (m messagesWindow) Less(i, j int) bool { return m[i].less(m[j]) }
func (m messagesWindow) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

// MessagesWindow is a concurrent-safe slice of MessageAge.
type MessagesWindow struct {
	internalSlice messagesWindow
	sync.RWMutex
}

// getDifficulty returns the adaptive pow difficulty.
func (m *MessagesWindow) getDifficulty(recentMessages int) int {
	return BaseDifficulty + int(AdaptiveRate*float64(recentMessages))
}

// messagesBefore returns the number of recent messages before the given upperbound and the given time.
func (m *MessagesWindow) messagesBefore(t time.Time, upperbound int) int {
	count := 0
	for i := 0; i < upperbound; i++ {
		if m.internalSlice[i].Timestamp.Add(time.Duration(ApowWindow) * time.Second).After(t) {
			count++
		}
	}
	return count
}

// messagesBeforeTime returns the number of recent messages before the given time.
func (m *MessagesWindow) messagesBeforeTime(t time.Time) int {
	return m.messagesBefore(t, m.timePosition(t))
}

// messagesBeforeMsg returns the number of recent messages before the given msg and its position within the MessagesWindow.
func (m *MessagesWindow) messagesBeforeMsg(msg MessageAge) (int, int) {
	p := m.lexicalPosition(msg)

	return m.messagesBefore(msg.Timestamp, p), p
}

// timePosition returns the position of the given timestamp within the MessagesWindow.
func (m *MessagesWindow) timePosition(t time.Time) int {
	sort.Sort(m.internalSlice)

	for i, v := range m.internalSlice {
		if t.Before(v.Timestamp) {
			return i
		}
	}
	return len(m.internalSlice)
}

// lexicalPosition returns the position of the given msg within the MessagesWindow.
func (m *MessagesWindow) lexicalPosition(msg MessageAge) int {
	sort.Sort(m.internalSlice)

	for i, v := range m.internalSlice {
		if msg.less(v) {
			return i
		}
	}
	return len(m.internalSlice)
}

// insert inserts the given msg at the given position.
func (m *MessagesWindow) insert(msg MessageAge, position int) {
	switch position {
	case 0:
		m.internalSlice = append(messagesWindow{msg}, m.internalSlice...)
	case len(m.internalSlice):
		m.internalSlice = append(m.internalSlice, msg)
	default:
		m.internalSlice = append(m.internalSlice[:position], append(messagesWindow{msg}, m.internalSlice[position:]...)...)
	}
}

// clean removes messages older than the ApowWindow.
func (m *MessagesWindow) clean() {
	l := m.internalSlice.Len()
	if l <= 1 {
		return
	}

	last := m.internalSlice[l-1].Timestamp

	for i := l - 2; i >= 0; i-- {
		if m.internalSlice[i].Timestamp.Add(time.Duration(ApowWindow)*time.Second).Before(last) ||
			m.internalSlice[i].Timestamp.Add(time.Duration(ApowWindow)*time.Second).Equal(last) {
			m.internalSlice = m.internalSlice[i+1:]
			return
		}
	}
}

// Append adds a new messageAge and eventually cleans the messagesWindow from older elements.
func (m *MessagesWindow) Append(msg MessageAge) {
	m.Lock()
	defer m.Unlock()

	m.internalSlice = append(m.internalSlice, msg)
	m.clean()
}

// AdaptiveDifficulty returns the adaptive proof-of-work difficulty.
func (m *MessagesWindow) AdaptiveDifficulty(t time.Time) int {
	m.Lock()
	defer m.Unlock()

	return m.getDifficulty(m.messagesBeforeTime(t))
}

// CheckDifficulty atomically checks the correctness of the pow difficult and updates the messagesWindow.
func (m *MessagesWindow) CheckDifficulty(msg MessageAge, d int) bool {
	m.Lock()
	defer m.Unlock()

	recentMessages, p := m.messagesBeforeMsg(msg)

	if m.getDifficulty(recentMessages) <= d {
		m.insert(msg, p)
		m.clean()
		return true
	}

	return false
}

// Len returns the len of the internal slice.
func (m *MessagesWindow) Len() int {
	m.RLock()
	defer m.RUnlock()
	return m.internalSlice.Len()
}
