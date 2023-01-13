package metrics

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
)

func TestBlockCountPerPayload(t *testing.T) {
	// it is empty initially
	require.Equal(t, BlockCountSinceStartPerComponentGrafana()[Attached], uint64(0))
	// simulate attaching 10 transaction payloads in 0s < t < 1s
	for i := 0; i < 10; i++ {
		increasePerComponentCounter(Attached)
		increasePerPayloadCounter(devnetvm.TransactionType)
	}
	require.Equal(t, BlockCountSinceStartPerComponentGrafana()[Attached], uint64(10))
	require.Equal(t, BlockCountSinceStartPerPayload(), map[payload.Type]uint64{devnetvm.TransactionType: 10})
}
