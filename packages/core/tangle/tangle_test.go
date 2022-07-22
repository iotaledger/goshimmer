package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

func TestTangleAttach(t *testing.T) {
	tangle := Tangle{
		Events:          *newEvents(),
		metadataStorage: memstorage.NewEpochStorage[BlockID, *BlockMetadata](),
		dbManager:       database.NewManager("/tmp/"),
	}

	tangle.Setup()

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
	assert.NoError(t, block1.DetermineID())
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
	assert.NoError(t, err)
	assert.NoError(t, block2.DetermineID())
	tangle.Events.BlockMissing.Hook(event.NewClosure[*BlockMissingEvent](func(evt *BlockMissingEvent) {
		t.Logf("block %s is missing", evt.BlockID)
	}))

	tangle.Events.BlockSolid.Hook(event.NewClosure[*BlockSolidEvent](func(evt *BlockSolidEvent) {
		t.Logf("block %s is solid", evt.BlockMetadata.ID())
	}))

	tangle.Events.MissingBlockStored.Hook(event.NewClosure[*MissingBlockStoredEvent](func(evt *MissingBlockStoredEvent) {
		t.Logf("missing block %s is stored", evt.BlockID)
	}))
	tangle.AttachBlock(block2)

	//assert some stuff

	tangle.AttachBlock(block1)

	event.Loop.WaitUntilAllTasksProcessed()

}
