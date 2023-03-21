package reentrantmutex

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLock(t *testing.T) {
	m := New()
	m.Lock(1)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		m.Lock(2)
		defer m.UnLock(2)

		fmt.Print("DONE 2")

		wg.Done()
	}()

	m.Lock(1)
	m.UnLock(1)

	time.Sleep(1 * time.Second)

	m.UnLock(1)

	fmt.Print("DONE 1")

	wg.Wait()
}

func TestRLock(t *testing.T) {
	m := New()
	m.RLock(1)
	m.RLock(2)
	m.RLock(3)
	m.RUnlock(2)

	var lockAcquired bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		m.Lock(1)
		m.UnLock(1)

		lockAcquired = true

		wg.Done()
	}()

	time.Sleep(1 * time.Second)

	require.False(t, lockAcquired)

	m.RUnlock(3)
	m.RUnlock(1)

	wg.Wait()

	require.True(t, lockAcquired)
}
