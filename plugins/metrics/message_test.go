package metrics

import (
	"testing"

	valuepayload "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
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
		increasePerPayloadCounter(drng.PayloadType)
	}
	assert.Equal(t, MessageTotalCountSinceStart(), (uint64)(15))
	assert.Equal(t, MessageCountSinceStartPerPayload(), map[payload.Type]uint64{valuepayload.Type: 10, drng.PayloadType: 5})
}
