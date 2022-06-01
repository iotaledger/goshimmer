package metrics

import (
	"testing"

	"github.com/magiconair/properties/assert"

	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

func TestMessageCountPerPayload(t *testing.T) {
	// it is empty initially
	assert.Equal(t, MessageCountSinceStartPerComponentGrafana()[Store], uint64(0))
	// simulate attaching 10 transaction payloads in 0s < t < 1s
	for i := 0; i < 10; i++ {
		increasePerComponentCounter(Store)
		increasePerPayloadCounter(devnetvm.TransactionType)
	}
	assert.Equal(t, MessageCountSinceStartPerComponentGrafana()[Store], uint64(10))
	assert.Equal(t, MessageCountSinceStartPerPayload(), map[payload.Type]uint64{devnetvm.TransactionType: 10})
	// simulate attaching 5 drng payloads
	for i := 0; i < 5; i++ {
		increasePerComponentCounter(Store)
	}
	assert.Equal(t, MessageCountSinceStartPerComponentGrafana()[Store], uint64(15))
	assert.Equal(t, MessageCountSinceStartPerPayload(), map[payload.Type]uint64{devnetvm.TransactionType: 10})
}
