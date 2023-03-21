package reentrantmutex

import "sync"

type ThreadID uint64

var (
	threadIDCounter      ThreadID
	threadIDCounterMutex sync.Mutex
)

func NewThreadID() ThreadID {
	threadIDCounterMutex.Lock()
	defer threadIDCounterMutex.Unlock()

	threadIDCounter++

	return threadIDCounter
}
