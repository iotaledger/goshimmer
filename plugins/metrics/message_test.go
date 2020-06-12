package metrics

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tipselector"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/events"
	"github.com/magiconair/properties/assert"
)

func TestReceivedMessagesPerSecond(t *testing.T) {
	// simulate attaching 10 value payloads in 0s < t < 1s
	for i := 0; i < 10; i++ {
		increaseReceivedMPSCounter()
	}

	// first measurement happens at t=1s
	measureReceivedMPS()

	// simulate 5 TPS for 1s < t < 2s
	for i := 0; i < 5; i++ {
		increaseReceivedMPSCounter()
	}

	assert.Equal(t, MPS(), (uint64)(10))
	// measure at t=2s
	measureReceivedMPS()
	assert.Equal(t, MPS(), (uint64)(5))
	// measure at t=3s
	measureReceivedMPS()
	assert.Equal(t, MPS(), (uint64)(0))
}

func TestReceivedMPSUpdatedEvent(t *testing.T) {
	var wg sync.WaitGroup
	Events.ReceivedMPSUpdated.Attach(events.NewClosure(func(mps uint64) {
		assert.Equal(t, mps, (uint64)(10))
		wg.Done()
	}))
	// simulate attaching 10 value payloads in 0s < t < 1s
	for i := 0; i < 10; i++ {
		increaseReceivedMPSCounter()
	}
	wg.Add(1)
	measureReceivedMPS()
	wg.Wait()
}

func TestMessageTips(t *testing.T) {
	var wg sync.WaitGroup
	// messagelayer TipSelector not configured here, so to avoid nil pointer panic, we instantiate it
	messagelayer.TipSelector = tipselector.New()
	metrics.Events().MessageTips.Attach(events.NewClosure(func(tips uint64) {
		atomic.StoreUint64(&messageTips, tips)
		wg.Done()
	}))
	wg.Add(1)
	measureMessageTips()
	wg.Wait()
	assert.Equal(t, MessageTips(), (uint64)(0))
}
