package metrics

import (
	"testing"

	"github.com/magiconair/properties/assert"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
)

func TestBlockCountPerPayload(t *testing.T) {
	// it is empty initially
	assert.Equal(t, BlockCountSinceStartPerComponentGrafana()[Store], uint64(0))
	// simulate attaching 10 transaction payloads in 0s < t < 1s
	for i := 0; i < 10; i++ {
		increasePerComponentCounter(Store)
		increasePerPayloadCounter(devnetvm.TransactionType)
	}
	assert.Equal(t, BlockCountSinceStartPerComponentGrafana()[Store], uint64(10))
	assert.Equal(t, BlockCountSinceStartPerPayload(), map[payload.Type]uint64{devnetvm.TransactionType: 10})
}
