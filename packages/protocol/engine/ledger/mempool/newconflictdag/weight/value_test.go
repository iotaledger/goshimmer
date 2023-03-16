package weight

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
)

func TestValue(t *testing.T) {
	value := NewValue(1, 2, acceptance.Accepted)

	require.Equal(t, int64(1), value.CumulativeWeight())
	require.Equal(t, int64(2), value.ValidatorsWeight())
	require.Equal(t, acceptance.Accepted, value.AcceptanceState())

	newValue := value.SetAcceptanceState(acceptance.Rejected)

	require.Equal(t, int64(1), value.CumulativeWeight())
	require.Equal(t, int64(2), value.ValidatorsWeight())
	require.Equal(t, acceptance.Accepted, value.AcceptanceState())

	require.Equal(t, int64(1), newValue.CumulativeWeight())
	require.Equal(t, int64(2), newValue.ValidatorsWeight())
	require.Equal(t, acceptance.Rejected, newValue.AcceptanceState())
}
