package blockdag

import (
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/runtime/module"
)

type BlockDAG interface {
	Events() *Events

	// Attach is used to attach new Blocks to the BlockDAG. It is the main function of the BlockDAG that triggers Events.
	Attach(data *models.Block) (block *Block, wasAttached bool, err error)

	// Block retrieves a Block with metadata from the in-memory storage of the BlockDAG.
	Block(id models.BlockID) (block *Block, exists bool)

	// SetOrphaned marks a Block as orphaned and propagates it to its future cone.
	SetOrphaned(block *Block, orphaned bool) (updated bool)

	// SetInvalid marks a Block as invalid and propagates the invalidity to its future cone.
	SetInvalid(block *Block, reason error) (wasUpdated bool)

	module.Interface
}
