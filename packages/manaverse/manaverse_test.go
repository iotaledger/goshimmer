package manaverse

import (
	"fmt"
	"testing"
	"time"
)

func Test(t *testing.T) {
	scheduler := NewScheduler()
	scheduler.Start()

	time.Sleep(20 * time.Second)

	fmt.Println(scheduler.iterations)
}
