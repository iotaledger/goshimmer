package models

import (
	"fmt"
	"sync/atomic"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	blocksByAlias  map[string]*Block
	sequenceNumber uint64
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	return options.Apply(&TestFramework{
		blocksByAlias: make(map[string]*Block),
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

	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	block.ID().RegisterAlias(alias)

	t.blocksByAlias[alias] = block

	return
}

// Block retrieves the Blocks that is associated with the given alias.
func (t *TestFramework) Block(alias string) (block *Block) {
	block, ok := t.blocksByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Block alias %s not registered", alias))
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

func GenesisRootBlockProvider(index epoch.Index) *set.AdvancedSet[BlockID] {
	return set.NewAdvancedSet[BlockID](EmptyBlockID)
}
