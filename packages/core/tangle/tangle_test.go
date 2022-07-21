package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

func TestTangleAttach(t *testing.T) {
	tangle := Tangle{
		Events:          *NewEvents(),
		metadataStorage: memstorage.NewEpochStorage[BlockID, *BlockMetadata](),
		dbManager:       database.NewManager("/tmp/"),
	}

	refs := NewParentBlockIDs()
	refs.AddStrong(EmptyBlockID)

	block1, err := NewBlockWithValidation(
		refs,
		time.Now(),
		ed25519.PublicKey{},
		0,
		payload.NewGenericDataPayload([]byte("")),
		0,
		ed25519.Signature{},
		0,
		epoch.NewECRecord(0),
		BlockVersion,
	)
	block1.DetermineID()
	assert.NoError(t, err)

	refs2 := NewParentBlockIDs()
	refs2.AddStrong(block1.ID())

	block2, err := NewBlockWithValidation(
		refs2,
		time.Now(),
		ed25519.PublicKey{},
		0,
		payload.NewGenericDataPayload([]byte("")),
		0,
		ed25519.Signature{},
		0,
		epoch.NewECRecord(0),
		BlockVersion,
	)
	block2.DetermineID()

	assert.NoError(t, err)
	tangle.Attach(block2)

	//assert some stuff

	tangle.Attach(block1)

}
