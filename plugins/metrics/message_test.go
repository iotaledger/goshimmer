package metrics

import (
	"sync"
	"testing"

	valuepayload "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	drngpayload "github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/events"
	"github.com/magiconair/properties/assert"
)

func TestMessageCountPerPayload(t *testing.T) {
	// it is empty initially
	assert.Equal(t, MessageTotalCountSinceStart(), (uint64)(0))
	// simulate attaching 10 value payloads in 0s < t < 1s
	for i := 0; i < 10; i++ {
		increasePerPayloadCounter(valuepayload.Type)
	}
	assert.Equal(t, MessageTotalCountSinceStart(), (uint64)(10))
	assert.Equal(t, MessageCountSinceStartPerPayload(), map[payload.Type]uint64{valuepayload.Type: 10})
	// simulate attaching 5 drng payloads
	for i := 0; i < 5; i++ {
		increasePerPayloadCounter(drngpayload.Type)
	}
	assert.Equal(t, MessageTotalCountSinceStart(), (uint64)(15))
	assert.Equal(t, MessageCountSinceStartPerPayload(), map[payload.Type]uint64{valuepayload.Type: 10, drngpayload.Type: 5})
}

func TestMessageTips(t *testing.T) {
	var wg sync.WaitGroup
	// messagelayer TipSelector not configured here, so to avoid nil pointer panic, we instantiate it
	messagelayer.TipSelector()
	metrics.Events().MessageTips.Attach(events.NewClosure(func(tips uint64) {
		messageTips.Store(tips)
		wg.Done()
	}))
	wg.Add(1)
	measureMessageTips()
	wg.Wait()
	assert.Equal(t, MessageTips(), (uint64)(0))
}
