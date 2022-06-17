package manaverse

import (
	"testing"
	"time"
)

func Test(t *testing.T) {
	scheduler := NewScheduler()
	if false {
		scheduler.Push(nil)
	}

	time.Sleep(20 * time.Second)
}
