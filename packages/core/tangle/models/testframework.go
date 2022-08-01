package models

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/options"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

var (
	selfLocalIdentity = identity.GenerateLocalIdentity()
	selfNode          = identity.New(selfLocalIdentity.PublicKey())
)

// TestFramework implements a framework for conveniently issuing blocks in a tangle as part of unit tests in a
// simplified way.
type TestFramework struct {
	blocksByAlias  map[string]*Block
	options        *BlockTestFrameworkOptions
	sequenceNumber uint64
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(opts ...options.Option[BlockTestFrameworkOptions]) (blockTestFramework *TestFramework) {
	blockTestFramework = &TestFramework{
		blocksByAlias: make(map[string]*Block),
		options:       NewBlockTestFrameworkOptions(opts...),
	}

	return
}

// CreateBlock creates a Block with the given alias and BlockTestFrameworkBlockOptions.
func (t *TestFramework) CreateBlock(alias string, blockFrameworkOptions ...options.Option[BlockTestFrameworkBlockOptions]) (block *Block) {
	oldOpts := NewBlockTestFrameworkBlockOptions(blockFrameworkOptions...)

	references := NewParentBlockIDs()
	if parents := t.strongParentIDs(oldOpts); len(parents) > 0 {
		references.AddAll(StrongParentType, parents)
	}
	if parents := t.weakParentIDs(oldOpts); len(parents) > 0 {
		references.AddAll(WeakParentType, parents)
	}
	if parents := t.shallowLikeParentIDs(oldOpts); len(parents) > 0 {
		references.AddAll(ShallowLikeParentType, parents)
	}

	opts := []options.Option[Block]{
		WithIssuer(oldOpts.issuer),
		WithParents(references),
	}

	if !oldOpts.issuingTime.IsZero() {
		opts = append(opts, WithIssuingTime(oldOpts.issuingTime))
	}

	if oldOpts.reattachmentBlockAlias != "" {
		opts = append(opts, WithPayload(t.Block(oldOpts.reattachmentBlockAlias).Payload()))
	} else {
		opts = append(opts, WithPayload(payload.NewGenericDataPayload([]byte(alias))))
	}

	block = NewBlock(0, ed25519.Signature{}, oldOpts.latestConfirmedEpoch, oldOpts.ecRecord, opts...)
	if block.SequenceNumber() == 0 {
		block = options.Apply(block, []options.Option[Block]{WithSequenceNumber(t.increaseSequenceNumber())})
	}

	if err := block.DetermineID(); err != nil {
		panic(err)
	}

	block.ID().RegisterAlias(alias)

	t.blocksByAlias[alias] = block

	return block
}

// IssueBlocks stores the given Blocks in the Storage and triggers the processing by the Tangle.
func (t *TestFramework) IssueBlocks(issueCallback func(block *Block), blockAliases ...string) *TestFramework {
	for _, blockAlias := range blockAliases {
		block := t.blocksByAlias[blockAlias]

		event.Loop.Submit(func() { issueCallback(block) })
	}

	return t
}

// WaitUntilAllTasksProcessed waits until all tasks are processed.
func (t *TestFramework) WaitUntilAllTasksProcessed() (self *TestFramework) {
	// time.Sleep(100 * time.Millisecond)
	event.Loop.WaitUntilAllTasksProcessed()
	return t
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

// strongParentIDs returns the BlockIDs that were defined to be the strong parents of the
// BlockTestFrameworkBlockOptions.
func (t *TestFramework) strongParentIDs(opts *BlockTestFrameworkBlockOptions) BlockIDs {
	return t.parentIDsByBlockAlias(opts.strongParents)
}

// weakParentIDs returns the BlockIDs that were defined to be the weak parents of the
// BlockTestFrameworkBlockOptions.
func (t *TestFramework) weakParentIDs(opts *BlockTestFrameworkBlockOptions) BlockIDs {
	return t.parentIDsByBlockAlias(opts.weakParents)
}

// shallowLikeParentIDs returns the BlockIDs that were defined to be the shallow like parents of the
// BlockTestFrameworkBlockOptions.
func (t *TestFramework) shallowLikeParentIDs(opts *BlockTestFrameworkBlockOptions) BlockIDs {
	return t.parentIDsByBlockAlias(opts.shallowLikeParents)
}

func (t *TestFramework) parentIDsByBlockAlias(parentAliases map[string]types.Empty) BlockIDs {
	parentIDs := NewBlockIDs()
	for parentAlias := range parentAliases {
		if parentAlias == "Genesis" {
			parentIDs.Add(EmptyBlockID)
			continue
		}

		parentIDs.Add(t.blocksByAlias[parentAlias].ID())
	}

	return parentIDs
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockTestFrameworkOptions ////////////////////////////////////////////////////////////////////////////////////

// BlockTestFrameworkOptions is a container that holds the values of all configurable options of the
// TestFramework.
type BlockTestFrameworkOptions struct {
	genesisOutputs        map[string]uint64
	coloredGenesisOutputs map[string]map[devnetvm.Color]uint64
}

// NewBlockTestFrameworkOptions is the constructor for the BlockTestFrameworkOptions.
func NewBlockTestFrameworkOptions(opts ...options.Option[BlockTestFrameworkOptions]) (frameworkOptions *BlockTestFrameworkOptions) {
	frameworkOptions = &BlockTestFrameworkOptions{
		genesisOutputs:        make(map[string]uint64),
		coloredGenesisOutputs: make(map[string]map[devnetvm.Color]uint64),
	}

	options.Apply(frameworkOptions, opts)

	return
}

// WithGenesisOutput returns a BlockTestFrameworkOption that defines a genesis Output that is loaded as part of the
// initial snapshot.
func WithGenesisOutput(alias string, balance uint64) options.Option[BlockTestFrameworkOptions] {
	return func(options *BlockTestFrameworkOptions) {
		if _, exists := options.genesisOutputs[alias]; exists {
			panic(fmt.Sprintf("duplicate genesis output alias (%s)", alias))
		}
		if _, exists := options.coloredGenesisOutputs[alias]; exists {
			panic(fmt.Sprintf("duplicate genesis output alias (%s)", alias))
		}

		options.genesisOutputs[alias] = balance
	}
}

// WithColoredGenesisOutput returns a BlockTestFrameworkOption that defines a genesis Output that is loaded as part of
// the initial snapshot and that supports colored coins.
func WithColoredGenesisOutput(alias string, balances map[devnetvm.Color]uint64) options.Option[BlockTestFrameworkOptions] {
	return func(options *BlockTestFrameworkOptions) {
		if _, exists := options.genesisOutputs[alias]; exists {
			panic(fmt.Sprintf("duplicate genesis output alias (%s)", alias))
		}
		if _, exists := options.coloredGenesisOutputs[alias]; exists {
			panic(fmt.Sprintf("duplicate genesis output alias (%s)", alias))
		}

		options.coloredGenesisOutputs[alias] = balances
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockTestFrameworkBlockOptions ///////////////////////////////////////////////////////////////////////////////

// BlockTestFrameworkBlockOptions is a struct that represents a collection of options that can be set when creating
// a Block with the TestFramework.
type BlockTestFrameworkBlockOptions struct {
	inputs                 map[string]types.Empty
	outputs                map[string]uint64
	coloredOutputs         map[string]map[devnetvm.Color]uint64
	strongParents          map[string]types.Empty
	weakParents            map[string]types.Empty
	shallowLikeParents     map[string]types.Empty
	issuer                 ed25519.PublicKey
	issuingTime            time.Time
	reattachmentBlockAlias string
	sequenceNumber         uint64
	overrideSequenceNumber bool
	ecRecord               *epoch.ECRecord
	latestConfirmedEpoch   epoch.Index
}

// NewBlockTestFrameworkBlockOptions is the constructor for the BlockTestFrameworkBlockOptions.
func NewBlockTestFrameworkBlockOptions(opts ...options.Option[BlockTestFrameworkBlockOptions]) (blockOptions *BlockTestFrameworkBlockOptions) {
	blockOptions = &BlockTestFrameworkBlockOptions{
		inputs:               make(map[string]types.Empty),
		outputs:              make(map[string]uint64),
		strongParents:        make(map[string]types.Empty),
		weakParents:          make(map[string]types.Empty),
		shallowLikeParents:   make(map[string]types.Empty),
		ecRecord:             epoch.NewECRecord(0),
		latestConfirmedEpoch: 0,
	}

	options.Apply(blockOptions, opts)

	return
}

// WithInputs returns a BlockOption that is used to provide the Inputs of the Transaction.
func WithInputs(inputAliases ...string) options.Option[BlockTestFrameworkBlockOptions] {
	return func(options *BlockTestFrameworkBlockOptions) {
		for _, inputAlias := range inputAliases {
			options.inputs[inputAlias] = types.Void
		}
	}
}

// WithOutput returns a BlockOption that is used to define a non-colored Output for the Transaction in the Block.
func WithOutput(alias string, balance uint64) options.Option[BlockTestFrameworkBlockOptions] {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.outputs[alias] = balance
	}
}

// WithColoredOutput returns a BlockOption that is used to define a colored Output for the Transaction in the Block.
func WithColoredOutput(alias string, balances map[devnetvm.Color]uint64) options.Option[BlockTestFrameworkBlockOptions] {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.coloredOutputs[alias] = balances
	}
}

// WithStrongParents returns a BlockOption that is used to define the strong parents of the Block.
func WithStrongParents(blockAliases ...string) options.Option[BlockTestFrameworkBlockOptions] {
	return func(options *BlockTestFrameworkBlockOptions) {
		for _, blockAlias := range blockAliases {
			options.strongParents[blockAlias] = types.Void
		}
	}
}

// WithWeakParents returns a BlockOption that is used to define the weak parents of the Block.
func WithWeakParents(blockAliases ...string) options.Option[BlockTestFrameworkBlockOptions] {
	return func(options *BlockTestFrameworkBlockOptions) {
		for _, blockAlias := range blockAliases {
			options.weakParents[blockAlias] = types.Void
		}
	}
}

// WithShallowLikeParents returns a BlockOption that is used to define the shallow like parents of the Block.
func WithShallowLikeParents(blockAliases ...string) options.Option[BlockTestFrameworkBlockOptions] {
	return func(options *BlockTestFrameworkBlockOptions) {
		for _, blockAlias := range blockAliases {
			options.shallowLikeParents[blockAlias] = types.Void
		}
	}
}

// WithReattachment returns a BlockOption that is used to select payload of which Block should be reattached.
func WithReattachment(blockAlias string) options.Option[BlockTestFrameworkBlockOptions] {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.reattachmentBlockAlias = blockAlias
	}
}

// WithECRecord returns a BlockOption that is used to define the ecr of the Block.
func WithECRecord(ecRecord *epoch.ECRecord) options.Option[BlockTestFrameworkBlockOptions] {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.ecRecord = ecRecord
	}
}

// WithLatestConfirmedEpoch returns a BlockOption that is used to define the latestConfirmedEpoch of the Block.
func WithLatestConfirmedEpoch(ei epoch.Index) options.Option[BlockTestFrameworkBlockOptions] {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.latestConfirmedEpoch = ei
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
