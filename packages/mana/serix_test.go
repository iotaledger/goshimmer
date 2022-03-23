package mana

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hive.go/serix"
)

func TestSerixPersistableBaseMana(t *testing.T) {
	baseMana := newPersistableMana()
	baseMana.NodeID = randomNodeID()
	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), baseMana)
	assert.NoError(t, err)
	assert.Equal(t, baseMana.Bytes(), serixBytes)
}
