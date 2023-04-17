package models

import (
	"fmt"
	"sync/atomic"

	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/runtime/options"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	slotTimeProviderFunc func() *slot.TimeProvider
	blocksByAlias        map[string]*Block
	sequenceNumber       uint64
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(slotTimeProviderFunc func() *slot.TimeProvider, opts ...options.Option[TestFramework]) *TestFramework {
	return options.Apply(&TestFramework{
		slotTimeProviderFunc: slotTimeProviderFunc,
		blocksByAlias:        make(map[string]*Block),
	}, opts)
}

// CreateBlock creates a Block with the given alias and BlockTestFrameworkBlockOptions.
func (t *TestFramework) CreateBlock(alias string, opts ...options.Option[Block]) (block *Block) {
	block = NewBlock(opts...)

	if block.SequenceNumber() == 0 {
		block.M.SequenceNumber = t.increaseSequenceNumber()
	}

	if block.ParentsCountByType(StrongParentType) == 0 {
		block.M.Parents.Add(StrongParentType, EmptyBlockID)
	}

	if err := block.DetermineID(t.slotTimeProviderFunc()); err != nil {
		panic(err)
	}
	block.ID().RegisterAlias(alias)

	t.blocksByAlias[alias] = block

	return
}

// CreateAndSignBlock creates a Block with the given alias and BlockTestFrameworkBlockOptions and signs it with the given keyPair.
func (t *TestFramework) CreateAndSignBlock(alias string, keyPair *ed25519.KeyPair, opts ...options.Option[Block]) (block *Block) {
	block = NewBlock(opts...)

	if block.SequenceNumber() == 0 {
		block.M.SequenceNumber = t.increaseSequenceNumber()
	}

	if block.ParentsCountByType(StrongParentType) == 0 {
		block.M.Parents.Add(StrongParentType, EmptyBlockID)
	}

	if err := block.Sign(keyPair); err != nil {
		panic(err)
	}

	if err := block.DetermineID(t.slotTimeProviderFunc()); err != nil {
		panic(err)
	}
	block.ID().RegisterAlias(alias)

	t.blocksByAlias[alias] = block

	return
}

// SetBlock set a Block with the given alias.
func (t *TestFramework) SetBlock(alias string, block *Block) {
	block.ID().RegisterAlias(alias)

	t.blocksByAlias[alias] = block
}

// Block retrieves the Blocks that is associated with the given alias.
func (t *TestFramework) Block(alias string) (block *Block) {
	block, ok := t.blocksByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Block alias %s not registered", alias))
	}

	return
}

// Blocks retrieves the Blocks that are associated with the give aliases.
func (t *TestFramework) Blocks(aliases ...string) (blocks []*Block) {
	blocks = make([]*Block, len(aliases))
	for i, alias := range aliases {
		blocks[i] = t.Block(alias)
	}

	return
}

// BlockIDs retrieves the Blocks that are associated with the given aliases.
func (t *TestFramework) BlockIDs(aliases ...string) (blockIDs BlockIDs) {
	blockIDs = NewBlockIDs()
	for _, alias := range aliases {
		blockIDs.Add(t.Block(alias).ID())
	}

	return
}

func (t *TestFramework) increaseSequenceNumber() (increasedSequenceNumber uint64) {
	return atomic.AddUint64(&t.sequenceNumber, 1) - 1
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBlock(alias string, block *Block) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.blocksByAlias[alias] = block
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func GenesisRootBlockProvider(index slot.Index) *advancedset.AdvancedSet[BlockID] {
	return advancedset.New(EmptyBlockID)
}
