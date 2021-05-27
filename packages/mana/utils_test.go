package mana

import (
	"testing"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

func TestNodeMap_GetPercentile(t *testing.T) {
	nodes := make(NodeMap)
	nodes[identity.GenerateIdentity().ID()] = 1
	nodes[identity.GenerateIdentity().ID()] = 2
	nodes[identity.GenerateIdentity().ID()] = 3
	checkID := identity.GenerateIdentity().ID()
	nodes[checkID] = 4
	percentile, err := nodes.GetPercentile(checkID)
	assert.NoError(t, err)
	assert.Equal(t, 75.0, percentile)
}
