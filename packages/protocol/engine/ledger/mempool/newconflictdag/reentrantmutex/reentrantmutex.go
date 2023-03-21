package reentrantmutex

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
)

type ReEntrantMutex struct {
	debugID       string
	wLockThreadID *ThreadID
	wLockCounter  int
	rLockCounters map[ThreadID]int
	mutexUnlocked sync.Cond
	mutex         sync.Mutex
}

func New(debugID string) *ReEntrantMutex {
	r := new(ReEntrantMutex)
	r.debugID = debugID
	r.rLockCounters = make(map[ThreadID]int)
	r.mutexUnlocked.L = &r.mutex

	return r
}
func (m *ReEntrantMutex) Origin() string {
	_, fileName, lineNumber, _ := runtime.Caller(2)

	return fileName + ":" + strconv.Itoa(lineNumber)
}

func (m *ReEntrantMutex) RLock(threadID ThreadID) {
	fmt.Printf("[RLOCKING]\t%s\t%04d\t%s\n", m.debugID, threadID, m.Origin())
	defer fmt.Printf("[RLOCKED]\t%s\t%04d\t%s\n", m.debugID, threadID, m.Origin())

	m.mutex.Lock()
	defer m.mutex.Unlock()

	for !m.isRLockable(threadID) {
		m.mutexUnlocked.Wait()
	}

	m.rLockCounters[threadID]++
}

func (m *ReEntrantMutex) RUnlock(threadID ThreadID) {
	fmt.Printf("[RUNLOCKING]\t%s\t%04d\t%s\n", m.debugID, threadID, m.Origin())
	defer fmt.Printf("[RUNLOCKED]\t%s\t%04d\t%s\n", m.debugID, threadID, m.Origin())

	if m.rUnlock(threadID) {
		m.mutexUnlocked.Broadcast()
	}
}

func (m *ReEntrantMutex) Lock(threadID ThreadID) {
	fmt.Printf("[LOCKING]\t%s\t%04d\t%s\n", m.debugID, threadID, m.Origin())
	defer fmt.Printf("[LOCKED]\t%s\t%04d\t%s\n", m.debugID, threadID, m.Origin())

	m.mutex.Lock()
	defer m.mutex.Unlock()

	for !m.isLockable(threadID) {
		m.mutexUnlocked.Wait()
	}

	m.wLockThreadID = &threadID
	m.wLockCounter++
}

func (m *ReEntrantMutex) UnLock(threadID ThreadID) {
	fmt.Printf("[UNLOCKING]\t%s\t%04d\t%s\n", m.debugID, threadID, m.Origin())
	defer fmt.Printf("[UNLOCKED]\t%s\t%04d\t%s\n", m.debugID, threadID, m.Origin())

	if m.unlock(threadID) {
		m.mutexUnlocked.Broadcast()
	}
}

func (m *ReEntrantMutex) isLockable(threadID ThreadID) bool {
	if m.wLockThreadID != nil {
		return *m.wLockThreadID == threadID
	}

	if len(m.rLockCounters) == 0 {
		return true
	}

	if len(m.rLockCounters) == 1 {
		for rLockThreadID := range m.rLockCounters {
			return rLockThreadID == threadID
		}
	}

	return false
}

func (m *ReEntrantMutex) unlock(threadID ThreadID) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.wLockThreadID == nil || *m.wLockThreadID != threadID {
		panic("threadID does not match current threadID")
	}

	m.wLockCounter--

	if m.wLockCounter == 0 {
		m.wLockThreadID = nil
		return true
	}

	return false
}

func (m *ReEntrantMutex) isRLockable(threadID ThreadID) bool {
	if m.wLockThreadID != nil {
		return *m.wLockThreadID == threadID
	}

	return true
}

func (m *ReEntrantMutex) rUnlock(threadID ThreadID) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if counter, exists := m.rLockCounters[threadID]; !exists || counter == 0 {
		panic("trying to RUnlock a threadID that was not locked before")
	}

	m.rLockCounters[threadID]--

	if m.rLockCounters[threadID] == 0 {
		delete(m.rLockCounters, threadID)
	}

	return true
}
