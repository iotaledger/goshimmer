package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActiveNodesMarshalling(t *testing.T) {
	nodes := map[string]identity.ID{
		"node1": identity.GenerateIdentity().ID(),
		"node2": identity.GenerateIdentity().ID(),
		"node3": identity.GenerateIdentity().ID(),
	}

	activeNodes := make(map[identity.ID]*ActivityLog)
	for _, nodeID := range nodes {
		a := NewActivityLog()

		for i := 0; i < crypto.Randomness.Intn(100); i++ {
			a.Add(time.Now().Add(time.Duration(i)*time.Minute + time.Hour))
		}

		activeNodes[nodeID] = a
	}

	activeNodesBytes := activeNodesToBytes(activeNodes)
	activeNodes2, err := activeNodesFromBytes(activeNodesBytes)
	require.NoError(t, err)

	for nodeID, a := range activeNodes {
		assert.EqualValues(t, a.Bytes(), activeNodes2[nodeID].Bytes())
	}
}
