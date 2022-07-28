package tangle

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/options"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
)

// region BlockTestFramework ///////////////////////////////////////////////////////////////////////////////////////////

var (
	selfLocalIdentity = identity.GenerateLocalIdentity()
	selfNode          = identity.New(selfLocalIdentity.PublicKey())
)

// BlockTestFramework implements a framework for conveniently issuing blocks in a tangle as part of unit tests in a
// simplified way.
type BlockTestFramework struct {
	tangle        *Tangle
	blocksByAlias map[string]*models.Block
	options       *BlockTestFrameworkOptions
}

// NewBlockTestFramework is the constructor of the BlockTestFramework.
func NewBlockTestFramework(tangle *Tangle, opts ...options.Option[BlockTestFrameworkOptions]) (blockTestFramework *BlockTestFramework) {
	blockTestFramework = &BlockTestFramework{
		tangle:        tangle,
		blocksByAlias: make(map[string]*models.Block),
		options:       NewBlockTestFrameworkOptions(opts...),
	}

	return
}

// CreateBlock creates a Block with the given alias and BlockTestFrameworkBlockOptions.
func (m *BlockTestFramework) CreateBlock(blockAlias string, blockOptions ...options.Option[BlockTestFrameworkBlockOptions]) (blk *models.Block) {
	opts := NewBlockTestFrameworkBlockOptions(blockOptions...)

	references := models.NewParentBlockIDs()

	if parents := m.strongParentIDs(opts); len(parents) > 0 {
		references.AddAll(models.StrongParentType, parents)
	}
	if parents := m.weakParentIDs(opts); len(parents) > 0 {
		references.AddAll(models.WeakParentType, parents)
	}
	if parents := m.shallowLikeParentIDs(opts); len(parents) > 0 {
		references.AddAll(models.ShallowLikeParentType, parents)
	}

	if opts.reattachmentBlockAlias != "" {
		reattachmentPayload := m.Block(opts.reattachmentBlockAlias).Payload()
		m.blocksByAlias[blockAlias] = newTestParentsPayloadBlockWithOptions(reattachmentPayload, references, opts)
	} else {
		m.blocksByAlias[blockAlias] = newTestParentsDataBlockWithOptions(blockAlias, references, opts)
	}

	if err := m.blocksByAlias[blockAlias].DetermineID(); err != nil {
		panic(err)
	}

	m.blocksByAlias[blockAlias].ID().RegisterAlias(blockAlias)

	return m.blocksByAlias[blockAlias]
}

// IssueBlocks stores the given Blocks in the Storage and triggers the processing by the Tangle.
func (m *BlockTestFramework) IssueBlocks(blockAliases ...string) *BlockTestFramework {
	for _, blockAlias := range blockAliases {
		currentBlockAlias := blockAlias

		event.Loop.Submit(func() {
			m.tangle.AttachBlock(m.blocksByAlias[currentBlockAlias])
		})
	}

	return m
}

// WaitUntilAllTasksProcessed waits until all tasks are processed.
func (m *BlockTestFramework) WaitUntilAllTasksProcessed() (self *BlockTestFramework) {
	// time.Sleep(100 * time.Millisecond)
	event.Loop.WaitUntilAllTasksProcessed()
	return m
}

// Block retrieves the Blocks that is associated with the given alias.
func (m *BlockTestFramework) Block(alias string) (block *models.Block) {
	block, ok := m.blocksByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Block alias %s not registered", alias))
	}
	return
}

// BlockIDs retrieves the Blocks that are associated with the given aliases.
func (m *BlockTestFramework) BlockIDs(aliases ...string) (blockIDs models.BlockIDs) {
	blockIDs = models.NewBlockIDs()
	for _, alias := range aliases {
		blockIDs.Add(m.Block(alias).ID())
	}
	return
}

// strongParentIDs returns the BlockIDs that were defined to be the strong parents of the
// BlockTestFrameworkBlockOptions.
func (m *BlockTestFramework) strongParentIDs(opts *BlockTestFrameworkBlockOptions) models.BlockIDs {
	return m.parentIDsByBlockAlias(opts.strongParents)
}

// weakParentIDs returns the BlockIDs that were defined to be the weak parents of the
// BlockTestFrameworkBlockOptions.
func (m *BlockTestFramework) weakParentIDs(opts *BlockTestFrameworkBlockOptions) models.BlockIDs {
	return m.parentIDsByBlockAlias(opts.weakParents)
}

// shallowLikeParentIDs returns the BlockIDs that were defined to be the shallow like parents of the
// BlockTestFrameworkBlockOptions.
func (m *BlockTestFramework) shallowLikeParentIDs(opts *BlockTestFrameworkBlockOptions) models.BlockIDs {
	return m.parentIDsByBlockAlias(opts.shallowLikeParents)
}

func (m *BlockTestFramework) parentIDsByBlockAlias(parentAliases map[string]types.Empty) models.BlockIDs {
	parentIDs := models.NewBlockIDs()
	for parentAlias := range parentAliases {
		if parentAlias == "Genesis" {
			parentIDs.Add(models.EmptyBlockID)
			continue
		}

		parentIDs.Add(m.blocksByAlias[parentAlias].ID())
	}

	return parentIDs
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockTestFrameworkOptions ////////////////////////////////////////////////////////////////////////////////////

// BlockTestFrameworkOptions is a container that holds the values of all configurable options of the
// BlockTestFramework.
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
// a Block with the BlockTestFramework.
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

// WithIssuer returns a BlockOption that is used to define the issuer of the Block.
func WithIssuer(issuer ed25519.PublicKey) options.Option[BlockTestFrameworkBlockOptions] {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.issuer = issuer
	}
}

// WithIssuingTime returns a BlockOption that is used to set issuing time of the Block.
func WithIssuingTime(issuingTime time.Time) options.Option[BlockTestFrameworkBlockOptions] {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.issuingTime = issuingTime
	}
}

// WithReattachment returns a BlockOption that is used to select payload of which Block should be reattached.
func WithReattachment(blockAlias string) options.Option[BlockTestFrameworkBlockOptions] {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.reattachmentBlockAlias = blockAlias
	}
}

// WithSequenceNumber returns a BlockOption that is used to define the sequence number of the Block.
func WithSequenceNumber(sequenceNumber uint64) options.Option[BlockTestFrameworkBlockOptions] {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.sequenceNumber = sequenceNumber
		options.overrideSequenceNumber = true
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

// region Utility functions ////////////////////////////////////////////////////////////////////////////////////////////

// NewTestTangle returns a Tangle instance with a testing schedulerConfig.
func NewTestTangle() *Tangle {
	t := New(WithDBManagerPath("/tmp/"))

	t.Events.Error.Hook(event.NewClosure(func(e error) {
		fmt.Println(e)
	}))

	return t
}

var _sequenceNumber uint64

func nextSequenceNumber() uint64 {
	return atomic.AddUint64(&_sequenceNumber, 1) - 1
}

func newTestNonceBlock(nonce uint64) *models.Block {
	block := models.NewBlock(models.NewParentBlockIDs().AddStrong(models.EmptyBlockID),
		time.Time{}, ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("test")), nonce, ed25519.Signature{}, 0, epoch.NewECRecord(0))

	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return block
}

func newTestDataBlock(payloadString string) *models.Block {
	block := models.NewBlock(models.NewParentBlockIDs().AddStrong(models.EmptyBlockID),
		time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{}, 0, epoch.NewECRecord(0))

	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return block
}

func newTestDataBlockPublicKey(payloadString string, publicKey ed25519.PublicKey) *models.Block {
	block := models.NewBlock(models.NewParentBlockIDs().AddStrong(models.EmptyBlockID),
		time.Now(), publicKey, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{}, 0, epoch.NewECRecord(0))

	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return block
}

func newTestParentsDataBlock(payloadString string, references models.ParentBlockIDs) (block *models.Block) {
	block = models.NewBlock(references, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{}, 0, epoch.NewECRecord(0))

	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return
}

func newTestParentsDataBlockWithOptions(payloadString string, references models.ParentBlockIDs, opts *BlockTestFrameworkBlockOptions) (block *models.Block) {
	var sequenceNumber uint64
	if opts.overrideSequenceNumber {
		sequenceNumber = opts.sequenceNumber
	} else {
		sequenceNumber = nextSequenceNumber()
	}
	if opts.issuingTime.IsZero() {
		block = models.NewBlock(references, time.Now(), opts.issuer, sequenceNumber, payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{}, opts.latestConfirmedEpoch, opts.ecRecord)
	} else {
		block = models.NewBlock(references, opts.issuingTime, opts.issuer, sequenceNumber, payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{}, opts.latestConfirmedEpoch, opts.ecRecord)
	}

	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return
}

func newTestParentsPayloadBlock(p payload.Payload, references models.ParentBlockIDs) (block *models.Block) {
	block = models.NewBlock(references, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), p, 0, ed25519.Signature{}, 0, nil)

	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return
}

func newTestParentsPayloadBlockWithOptions(p payload.Payload, references models.ParentBlockIDs, opts *BlockTestFrameworkBlockOptions) (block *models.Block) {
	var sequenceNumber uint64
	if opts.overrideSequenceNumber {
		sequenceNumber = opts.sequenceNumber
	} else {
		sequenceNumber = nextSequenceNumber()
	}
	var err error
	if opts.issuingTime.IsZero() {
		block = models.NewBlock(references, time.Now(), opts.issuer, sequenceNumber, p, 0, ed25519.Signature{}, opts.latestConfirmedEpoch, opts.ecRecord)
	} else {
		block = models.NewBlock(references, opts.issuingTime, opts.issuer, sequenceNumber, p, 0, ed25519.Signature{}, opts.latestConfirmedEpoch, opts.ecRecord)
	}
	if err != nil {
		panic(err)
	}
	if err = block.DetermineID(); err != nil {
		panic(err)
	}
	return
}

func newTestParentsPayloadWithTimestamp(p payload.Payload, references models.ParentBlockIDs, timestamp time.Time) *models.Block {
	block := models.NewBlock(references, timestamp, ed25519.PublicKey{}, nextSequenceNumber(), p, 0, ed25519.Signature{}, 0, nil)
	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return block
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
