package tangleold

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"

	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

// region BlockTestFramework /////////////////////////////////////////////////////////////////////////////////////////

// BlockTestFramework implements a framework for conveniently issuing blocks in a tangle as part of unit tests in a
// simplified way.
type BlockTestFramework struct {
	tangle                   *Tangle
	conflictIDs              map[string]utxo.TransactionID
	blocksByAlias            map[string]*Block
	walletsByAlias           map[string]wallet
	walletsByAddress         map[devnetvm.Address]wallet
	inputsByAlias            map[string]devnetvm.Input
	outputsByAlias           map[string]devnetvm.Output
	outputsByID              map[utxo.OutputID]devnetvm.Output
	options                  *BlockTestFrameworkOptions
	oldIncreaseIndexCallback markers.IncreaseIndexCallback
	snapshot                 *ledger.Snapshot
	outputCounter            uint16
}

// NewBlockTestFramework is the constructor of the BlockTestFramework.
func NewBlockTestFramework(tangle *Tangle, options ...BlockTestFrameworkOption) (blockTestFramework *BlockTestFramework) {
	blockTestFramework = &BlockTestFramework{
		tangle:           tangle,
		conflictIDs:      make(map[string]utxo.TransactionID),
		blocksByAlias:    make(map[string]*Block),
		walletsByAlias:   make(map[string]wallet),
		walletsByAddress: make(map[devnetvm.Address]wallet),
		inputsByAlias:    make(map[string]devnetvm.Input),
		outputsByAlias:   make(map[string]devnetvm.Output),
		outputsByID:      make(map[utxo.OutputID]devnetvm.Output),
		options:          NewBlockTestFrameworkOptions(options...),
	}

	blockTestFramework.createGenesisOutputs()

	return
}

// Snapshot returns the Snapshot of the test framework.
func (m *BlockTestFramework) Snapshot() (snapshot *ledger.Snapshot) {
	return m.snapshot
}

// RegisterConflictID registers a ConflictID from the given Blocks' transactions with the BlockTestFramework and
// also an alias when printing the ConflictID.
func (m *BlockTestFramework) RegisterConflictID(alias, blockAlias string) {
	conflictID := m.ConflictIDFromBlock(blockAlias)
	m.conflictIDs[alias] = conflictID
	conflictID.RegisterAlias(alias)
}

func (m *BlockTestFramework) RegisterTransactionID(alias, blockAlias string) {
	TxID := m.ConflictIDFromBlock(blockAlias)
	TxID.RegisterAlias(alias)
}

// ConflictID returns the ConflictID registered with the given alias.
func (m *BlockTestFramework) ConflictID(alias string) (conflictID utxo.TransactionID) {
	conflictID, ok := m.conflictIDs[alias]
	if !ok {
		panic("no conflict registered with such alias " + alias)
	}

	return
}

// ConflictIDs returns the ConflictIDs registered with the given aliases.
func (m *BlockTestFramework) ConflictIDs(aliases ...string) (conflictIDs utxo.TransactionIDs) {
	conflictIDs = set.NewAdvancedSet[utxo.TransactionID]()

	for _, alias := range aliases {
		conflictID, ok := m.conflictIDs[alias]
		if !ok {
			panic("no conflict registered with such alias " + alias)
		}
		conflictIDs.Add(conflictID)
	}

	return
}

// CreateBlock creates a Block with the given alias and BlockTestFrameworkBlockOptions.
func (m *BlockTestFramework) CreateBlock(blockAlias string, blockOptions ...BlockOption) (block *Block) {
	options := NewBlockTestFrameworkBlockOptions(blockOptions...)

	references := NewParentBlockIDs()

	if parents := m.strongParentIDs(options); len(parents) > 0 {
		references.AddAll(StrongParentType, parents)
	}
	if parents := m.weakParentIDs(options); len(parents) > 0 {
		references.AddAll(WeakParentType, parents)
	}
	if parents := m.shallowLikeParentIDs(options); len(parents) > 0 {
		references.AddAll(ShallowLikeParentType, parents)
	}

	if options.reattachmentBlockAlias != "" {
		reattachmentPayload := m.Block(options.reattachmentBlockAlias).Payload()
		m.blocksByAlias[blockAlias] = newTestParentsPayloadBlockWithOptions(reattachmentPayload, references, options)
	} else {
		transaction := m.buildTransaction(options)
		if transaction != nil {
			m.blocksByAlias[blockAlias] = newTestParentsPayloadBlockWithOptions(transaction, references, options)
		} else {
			m.blocksByAlias[blockAlias] = newTestParentsDataBlockWithOptions(blockAlias, references, options)
		}
	}

	if err := m.blocksByAlias[blockAlias].DetermineID(); err != nil {
		panic(err)
	}

	m.blocksByAlias[blockAlias].ID().RegisterAlias(blockAlias)

	return m.blocksByAlias[blockAlias]
}

// IncreaseMarkersIndexCallback is the IncreaseMarkersIndexCallback that the BlockTestFramework uses to determine when
// to assign new Markers to blocks.
func (m *BlockTestFramework) IncreaseMarkersIndexCallback(markers.SequenceID, markers.Index) bool {
	return false
}

// PreventNewMarkers disables the generation of new Markers for the given Blocks.
func (m *BlockTestFramework) PreventNewMarkers(enabled bool) *BlockTestFramework {
	if enabled && m.oldIncreaseIndexCallback == nil {
		m.oldIncreaseIndexCallback = m.tangle.Options.IncreaseMarkersIndexCallback
		m.tangle.Options.IncreaseMarkersIndexCallback = m.IncreaseMarkersIndexCallback
		return m
	}

	if !enabled && m.oldIncreaseIndexCallback != nil {
		m.tangle.Options.IncreaseMarkersIndexCallback = m.oldIncreaseIndexCallback
		m.oldIncreaseIndexCallback = nil
		return m
	}

	return m
}

// LatestCommitment gets the latest commitment.
func (m *BlockTestFramework) LatestCommitment() (ecRecord *epoch.ECRecord, latestConfirmedEpoch epoch.Index, err error) {
	return m.tangle.Options.CommitmentFunc()
}

// IssueBlocks stores the given Blocks in the Storage and triggers the processing by the Tangle.
func (m *BlockTestFramework) IssueBlocks(blockAliases ...string) *BlockTestFramework {
	for _, blockAlias := range blockAliases {
		currentBlockAlias := blockAlias

		event.Loop.Submit(func() {
			m.tangle.Storage.StoreBlock(m.blocksByAlias[currentBlockAlias])
		})
	}

	return m
}

func (m *BlockTestFramework) WaitUntilAllTasksProcessed() (self *BlockTestFramework) {
	// time.Sleep(100 * time.Millisecond)
	event.Loop.WaitUntilAllTasksProcessed()
	return m
}

// Block retrieves the Blocks that is associated with the given alias.
func (m *BlockTestFramework) Block(alias string) (block *Block) {
	block, ok := m.blocksByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Block alias %s not registered", alias))
	}
	return
}

// BlockIDs retrieves the Blocks that is associated with the given alias.
func (m *BlockTestFramework) BlockIDs(aliases ...string) (blockIDs BlockIDs) {
	blockIDs = NewBlockIDs()
	for _, alias := range aliases {
		blockIDs.Add(m.Block(alias).ID())
	}
	return
}

// BlockMetadata retrieves the BlockMetadata that is associated with the given alias.
func (m *BlockTestFramework) BlockMetadata(alias string) (blockMetadata *BlockMetadata) {
	m.tangle.Storage.BlockMetadata(m.blocksByAlias[alias].ID()).Consume(func(blkMetadata *BlockMetadata) {
		blockMetadata = blkMetadata
	})

	return
}

// TransactionID returns the TransactionID of the Transaction contained in the Block associated with the given alias.
func (m *BlockTestFramework) TransactionID(blockAlias string) utxo.TransactionID {
	blockPayload := m.blocksByAlias[blockAlias].Payload()
	tx, ok := blockPayload.(*devnetvm.Transaction)
	if !ok {
		panic(fmt.Sprintf("Block with alias '%s' does not contain a Transaction", blockAlias))
	}

	return tx.ID()
}

// Output retrieves the Output that is associated with the given alias.
func (m *BlockTestFramework) Output(alias string) (output devnetvm.Output) {
	output, ok := m.outputsByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Output alias %s not registered", alias))
	}
	return
}

// TransactionMetadata returns the transaction metadata of the transaction contained within the given block.
// Panics if the block's payload isn't a transaction.
func (m *BlockTestFramework) TransactionMetadata(blockAlias string) (txMeta *ledger.TransactionMetadata) {
	m.tangle.Ledger.Storage.CachedTransactionMetadata(m.TransactionID(blockAlias)).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
		txMeta = transactionMetadata
	})
	return
}

// Transaction returns the transaction contained within the given block.
// Panics if the block's payload isn't a transaction.
func (m *BlockTestFramework) Transaction(blockAlias string) (tx utxo.Transaction) {
	m.tangle.Ledger.Storage.CachedTransaction(m.TransactionID(blockAlias)).Consume(func(transaction utxo.Transaction) {
		tx = transaction
	})
	return
}

// OutputMetadata returns the given output metadata.
func (m *BlockTestFramework) OutputMetadata(outputID utxo.OutputID) (outMeta *ledger.OutputMetadata) {
	m.tangle.Ledger.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledger.OutputMetadata) {
		outMeta = outputMetadata
	})
	return
}

// ConflictIDFromBlock returns the ConflictID of the Transaction contained in the Block associated with the given alias.
func (m *BlockTestFramework) ConflictIDFromBlock(blockAlias string) utxo.TransactionID {
	blockPayload := m.blocksByAlias[blockAlias].Payload()
	tx, ok := blockPayload.(utxo.Transaction)
	if !ok {
		panic(fmt.Sprintf("Block with alias '%s' does not contain a Transaction", blockAlias))
	}

	return tx.ID()
}

// Conflict returns the conflict emerging from the transaction contained within the given block.
// This function thus only works on the block creating ledger.Conflict.
// Panics if the block's payload isn't a transaction.
func (m *BlockTestFramework) Conflict(blockAlias string) (b *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
	m.tangle.Ledger.ConflictDAG.Storage.CachedConflict(m.ConflictIDFromBlock(blockAlias)).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		b = conflict
	})
	return
}

// createGenesisOutputs initializes the Outputs that are used by the BlockTestFramework as the genesis.
func (m *BlockTestFramework) createGenesisOutputs() {
	if len(m.options.genesisOutputs) == 0 {
		return
	}

	manaPledgeID, err := identity.RandomIDInsecure()
	if err != nil {
		panic(err)
	}
	manaPledgeTime := time.Now()

	outputsWithMetadata := make([]*ledger.OutputWithMetadata, 0)

	for alias, balance := range m.options.genesisOutputs {
		outputWithMetadata := m.createOutput(alias, devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{devnetvm.ColorIOTA: balance}), manaPledgeID, manaPledgeTime)
		outputsWithMetadata = append(outputsWithMetadata, outputWithMetadata)
	}
	for alias, coloredBalances := range m.options.coloredGenesisOutputs {
		outputWithMetadata := m.createOutput(alias, devnetvm.NewColoredBalances(coloredBalances), manaPledgeID, manaPledgeTime)
		outputsWithMetadata = append(outputsWithMetadata, outputWithMetadata)
	}
	activeNodes := createActivityLog(manaPledgeTime, manaPledgeID)

	m.snapshot = ledger.NewSnapshot(outputsWithMetadata, activeNodes)
	loadSnapshotToLedger(m.tangle.Ledger, m.snapshot)
}

// createActivityLog create activity log and adds provided node for given time.
func createActivityLog(activityTime time.Time, nodeID identity.ID) epoch.SnapshotEpochActivity {
	ei := epoch.IndexFromTime(activityTime)
	activeNodes := make(epoch.SnapshotEpochActivity)
	activeNodes[ei] = epoch.NewSnapshotNodeActivity()
	activeNodes[ei].SetNodeActivity(nodeID, 1)
	return activeNodes
}

func (m *BlockTestFramework) createOutput(alias string, coloredBalances *devnetvm.ColoredBalances, manaPledgeID identity.ID, manaPledgeTime time.Time) (outputWithMetadata *ledger.OutputWithMetadata) {
	addressWallet := createWallets(1)[0]
	m.walletsByAlias[alias] = addressWallet
	m.walletsByAddress[addressWallet.address] = addressWallet

	output := devnetvm.NewSigLockedColoredOutput(coloredBalances, addressWallet.address)
	output.SetID(utxo.NewOutputID(utxo.EmptyTransactionID, m.outputCounter))
	m.outputCounter++

	outputWithMetadata = ledger.NewOutputWithMetadata(output.ID(), output, manaPledgeTime, manaPledgeID, manaPledgeID)
	m.outputsByAlias[alias] = output
	m.outputsByID[output.ID()] = output
	m.inputsByAlias[alias] = devnetvm.NewUTXOInput(output.ID())

	return outputWithMetadata
}

// buildTransaction creates a Transaction from the given BlockTestFrameworkBlockOptions. It returns nil if there are
// no Transaction related BlockTestFrameworkBlockOptions.
func (m *BlockTestFramework) buildTransaction(options *BlockTestFrameworkBlockOptions) (transaction *devnetvm.Transaction) {
	if len(options.inputs) == 0 || len(options.outputs) == 0 {
		return
	}

	inputs := make([]devnetvm.Input, 0)
	for inputAlias := range options.inputs {
		inputs = append(inputs, m.inputsByAlias[inputAlias])
	}

	outputs := make([]devnetvm.Output, 0)
	for alias, balance := range options.outputs {
		addressWallet := createWallets(1)[0]
		m.walletsByAlias[alias] = addressWallet
		m.walletsByAddress[addressWallet.address] = addressWallet

		m.outputsByAlias[alias] = devnetvm.NewSigLockedSingleOutput(balance, m.walletsByAlias[alias].address)

		outputs = append(outputs, m.outputsByAlias[alias])
	}
	for alias, balances := range options.coloredOutputs {
		addressWallet := createWallets(1)[0]
		m.walletsByAlias[alias] = addressWallet
		m.walletsByAddress[addressWallet.address] = addressWallet

		m.outputsByAlias[alias] = devnetvm.NewSigLockedColoredOutput(devnetvm.NewColoredBalances(balances), m.walletsByAlias[alias].address)

		outputs = append(outputs, m.outputsByAlias[alias])
	}

	transaction = makeTransaction(devnetvm.NewInputs(inputs...), devnetvm.NewOutputs(outputs...), m.outputsByID, m.walletsByAddress)
	for outputIndex, output := range transaction.Essence().Outputs() {
		for alias, aliasedOutput := range m.outputsByAlias {
			if aliasedOutput == output {
				output.SetID(utxo.NewOutputID(transaction.ID(), uint16(outputIndex)))

				m.outputsByID[output.ID()] = output
				m.inputsByAlias[alias] = devnetvm.NewUTXOInput(output.ID())

				break
			}
		}
	}

	return
}

// strongParentIDs returns the BlockIDs that were defined to be the strong parents of the
// BlockTestFrameworkBlockOptions.
func (m *BlockTestFramework) strongParentIDs(options *BlockTestFrameworkBlockOptions) BlockIDs {
	return m.parentIDsByBlockAlias(options.strongParents)
}

// weakParentIDs returns the BlockIDs that were defined to be the weak parents of the
// BlockTestFrameworkBlockOptions.
func (m *BlockTestFramework) weakParentIDs(options *BlockTestFrameworkBlockOptions) BlockIDs {
	return m.parentIDsByBlockAlias(options.weakParents)
}

// shallowLikeParentIDs returns the BlockIDs that were defined to be the shallow like parents of the
// BlockTestFrameworkBlockOptions.
func (m *BlockTestFramework) shallowLikeParentIDs(options *BlockTestFrameworkBlockOptions) BlockIDs {
	return m.parentIDsByBlockAlias(options.shallowLikeParents)
}

func (m *BlockTestFramework) parentIDsByBlockAlias(parentAliases map[string]types.Empty) BlockIDs {
	parentIDs := NewBlockIDs()
	for parentAlias := range parentAliases {
		if parentAlias == "Genesis" {
			parentIDs.Add(EmptyBlockID)
			continue
		}

		parentIDs.Add(m.blocksByAlias[parentAlias].ID())
	}

	return parentIDs
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockTestFrameworkOptions //////////////////////////////////////////////////////////////////////////////////

// BlockTestFrameworkOptions is a container that holds the values of all configurable options of the
// BlockTestFramework.
type BlockTestFrameworkOptions struct {
	genesisOutputs        map[string]uint64
	coloredGenesisOutputs map[string]map[devnetvm.Color]uint64
}

// NewBlockTestFrameworkOptions is the constructor for the BlockTestFrameworkOptions.
func NewBlockTestFrameworkOptions(options ...BlockTestFrameworkOption) (frameworkOptions *BlockTestFrameworkOptions) {
	frameworkOptions = &BlockTestFrameworkOptions{
		genesisOutputs:        make(map[string]uint64),
		coloredGenesisOutputs: make(map[string]map[devnetvm.Color]uint64),
	}

	for _, option := range options {
		option(frameworkOptions)
	}

	return
}

// BlockTestFrameworkOption is the type that is used for options that can be passed into the BlockTestFramework to
// configure its behavior.
type BlockTestFrameworkOption func(*BlockTestFrameworkOptions)

// WithGenesisOutput returns a BlockTestFrameworkOption that defines a genesis Output that is loaded as part of the
// initial snapshot.
func WithGenesisOutput(alias string, balance uint64) BlockTestFrameworkOption {
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
func WithColoredGenesisOutput(alias string, balances map[devnetvm.Color]uint64) BlockTestFrameworkOption {
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

// region BlockTestFrameworkBlockOptions ///////////////////////////////////////////////////////////////////////////

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
func NewBlockTestFrameworkBlockOptions(options ...BlockOption) (blockOptions *BlockTestFrameworkBlockOptions) {
	blockOptions = &BlockTestFrameworkBlockOptions{
		inputs:               make(map[string]types.Empty),
		outputs:              make(map[string]uint64),
		strongParents:        make(map[string]types.Empty),
		weakParents:          make(map[string]types.Empty),
		shallowLikeParents:   make(map[string]types.Empty),
		ecRecord:             epoch.NewECRecord(0),
		latestConfirmedEpoch: 0,
	}

	for _, option := range options {
		option(blockOptions)
	}

	return
}

// BlockOption is the type that is used for options that can be passed into the CreateBlock method to configure its
// behavior.
type BlockOption func(*BlockTestFrameworkBlockOptions)

// WithInputs returns a BlockOption that is used to provide the Inputs of the Transaction.
func WithInputs(inputAliases ...string) BlockOption {
	return func(options *BlockTestFrameworkBlockOptions) {
		for _, inputAlias := range inputAliases {
			options.inputs[inputAlias] = types.Void
		}
	}
}

// WithOutput returns a BlockOption that is used to define a non-colored Output for the Transaction in the Block.
func WithOutput(alias string, balance uint64) BlockOption {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.outputs[alias] = balance
	}
}

// WithColoredOutput returns a BlockOption that is used to define a colored Output for the Transaction in the Block.
func WithColoredOutput(alias string, balances map[devnetvm.Color]uint64) BlockOption {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.coloredOutputs[alias] = balances
	}
}

// WithStrongParents returns a BlockOption that is used to define the strong parents of the Block.
func WithStrongParents(blockAliases ...string) BlockOption {
	return func(options *BlockTestFrameworkBlockOptions) {
		for _, blockAlias := range blockAliases {
			options.strongParents[blockAlias] = types.Void
		}
	}
}

// WithWeakParents returns a BlockOption that is used to define the weak parents of the Block.
func WithWeakParents(blockAliases ...string) BlockOption {
	return func(options *BlockTestFrameworkBlockOptions) {
		for _, blockAlias := range blockAliases {
			options.weakParents[blockAlias] = types.Void
		}
	}
}

// WithShallowLikeParents returns a BlockOption that is used to define the shallow like parents of the Block.
func WithShallowLikeParents(blockAliases ...string) BlockOption {
	return func(options *BlockTestFrameworkBlockOptions) {
		for _, blockAlias := range blockAliases {
			options.shallowLikeParents[blockAlias] = types.Void
		}
	}
}

// WithIssuer returns a BlockOption that is used to define the issuer of the Block.
func WithIssuer(issuer ed25519.PublicKey) BlockOption {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.issuer = issuer
	}
}

// WithIssuingTime returns a BlockOption that is used to set issuing time of the Block.
func WithIssuingTime(issuingTime time.Time) BlockOption {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.issuingTime = issuingTime
	}
}

// WithReattachment returns a BlockOption that is used to select payload of which Block should be reattached.
func WithReattachment(blockAlias string) BlockOption {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.reattachmentBlockAlias = blockAlias
	}
}

// WithSequenceNumber returns a BlockOption that is used to define the sequence number of the Block.
func WithSequenceNumber(sequenceNumber uint64) BlockOption {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.sequenceNumber = sequenceNumber
		options.overrideSequenceNumber = true
	}
}

// WithECRecord returns a BlockOption that is used to define the ecr of the Block.
func WithECRecord(ecRecord *epoch.ECRecord) BlockOption {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.ecRecord = ecRecord
	}
}

// WithLatestConfirmedEpoch returns a BlockOption that is used to define the latestConfirmedEpoch of the Block.
func WithLatestConfirmedEpoch(ei epoch.Index) BlockOption {
	return func(options *BlockTestFrameworkBlockOptions) {
		options.latestConfirmedEpoch = ei
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Utility functions ////////////////////////////////////////////////////////////////////////////////////////////

var _sequenceNumber uint64

func nextSequenceNumber() uint64 {
	return atomic.AddUint64(&_sequenceNumber, 1) - 1
}

func randomTransactionID() (randomTransactionID utxo.TransactionID) {
	if err := randomTransactionID.FromRandomness(); err != nil {
		panic(err)
	}

	return randomTransactionID
}

func randomConflictID() (randomConflictID utxo.TransactionID) {
	if err := randomConflictID.FromRandomness(); err != nil {
		panic(err)
	}

	return randomConflictID
}

func randomResourceID() (randomConflictID utxo.OutputID) {
	if err := randomConflictID.FromRandomness(); err != nil {
		panic(err)
	}

	return randomConflictID
}

func newTestNonceBlock(nonce uint64) *Block {
	block := NewBlock(NewParentBlockIDs().AddStrong(EmptyBlockID),
		time.Time{}, ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("test")), nonce, ed25519.Signature{}, 0, epoch.NewECRecord(0))

	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return block
}

func newTestDataBlock(payloadString string) *Block {
	block := NewBlock(NewParentBlockIDs().AddStrong(EmptyBlockID),
		time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{}, 0, epoch.NewECRecord(0))

	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return block
}

func newTestDataBlockPublicKey(payloadString string, publicKey ed25519.PublicKey) *Block {
	block := NewBlock(NewParentBlockIDs().AddStrong(EmptyBlockID),
		time.Now(), publicKey, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{}, 0, epoch.NewECRecord(0))

	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return block
}

func newTestParentsDataBlock(payloadString string, references ParentBlockIDs) (block *Block) {
	block = NewBlock(references, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{}, 0, epoch.NewECRecord(0))

	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return
}

func newTestParentsDataBlockWithOptions(payloadString string, references ParentBlockIDs, options *BlockTestFrameworkBlockOptions) (block *Block) {
	var sequenceNumber uint64
	if options.overrideSequenceNumber {
		sequenceNumber = options.sequenceNumber
	} else {
		sequenceNumber = nextSequenceNumber()
	}
	if options.issuingTime.IsZero() {
		block = NewBlock(references, time.Now(), options.issuer, sequenceNumber, payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{}, options.latestConfirmedEpoch, options.ecRecord)
	} else {
		block = NewBlock(references, options.issuingTime, options.issuer, sequenceNumber, payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{}, options.latestConfirmedEpoch, options.ecRecord)
	}

	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return
}

func newTestParentsPayloadBlock(p payload.Payload, references ParentBlockIDs) (block *Block) {
	block = NewBlock(references, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), p, 0, ed25519.Signature{}, 0, nil)

	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return
}

func newTestParentsPayloadBlockWithOptions(p payload.Payload, references ParentBlockIDs, options *BlockTestFrameworkBlockOptions) (block *Block) {
	var sequenceNumber uint64
	if options.overrideSequenceNumber {
		sequenceNumber = options.sequenceNumber
	} else {
		sequenceNumber = nextSequenceNumber()
	}
	var err error
	if options.issuingTime.IsZero() {
		block = NewBlock(references, time.Now(), options.issuer, sequenceNumber, p, 0, ed25519.Signature{}, options.latestConfirmedEpoch, options.ecRecord)
	} else {
		block = NewBlock(references, options.issuingTime, options.issuer, sequenceNumber, p, 0, ed25519.Signature{}, options.latestConfirmedEpoch, options.ecRecord)
	}
	if err != nil {
		panic(err)
	}
	if err = block.DetermineID(); err != nil {
		panic(err)
	}
	return
}

func newTestParentsPayloadWithTimestamp(p payload.Payload, references ParentBlockIDs, timestamp time.Time) *Block {
	block := NewBlock(references, timestamp, ed25519.PublicKey{}, nextSequenceNumber(), p, 0, ed25519.Signature{}, 0, nil)
	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return block
}

type wallet struct {
	keyPair ed25519.KeyPair
	address *devnetvm.ED25519Address
}

func (w wallet) privateKey() ed25519.PrivateKey {
	return w.keyPair.PrivateKey
}

func (w wallet) publicKey() ed25519.PublicKey {
	return w.keyPair.PublicKey
}

func createWallets(n int) []wallet {
	wallets := make([]wallet, n)
	for i := 0; i < n; i++ {
		kp := ed25519.GenerateKeyPair()
		wallets[i] = wallet{
			kp,
			devnetvm.NewED25519Address(kp.PublicKey),
		}
	}
	return wallets
}

func (w wallet) sign(txEssence *devnetvm.TransactionEssence) *devnetvm.ED25519Signature {
	return devnetvm.NewED25519Signature(w.publicKey(), w.privateKey().Sign(lo.PanicOnErr(txEssence.Bytes())))
}

// addressFromInput retrieves the Address belonging to an Input by looking it up in the outputs that we have created for
// the tests.
func addressFromInput(input devnetvm.Input, outputsByID devnetvm.OutputsByID) devnetvm.Address {
	typeCastedInput, ok := input.(*devnetvm.UTXOInput)
	if !ok {
		panic("unexpected Input type")
	}

	switch referencedOutput := outputsByID[typeCastedInput.ReferencedOutputID()]; referencedOutput.Type() {
	case devnetvm.SigLockedSingleOutputType:
		typeCastedOutput, ok := referencedOutput.(*devnetvm.SigLockedSingleOutput)
		if !ok {
			panic("failed to type cast SigLockedSingleOutput")
		}

		return typeCastedOutput.Address()
	case devnetvm.SigLockedColoredOutputType:
		typeCastedOutput, ok := referencedOutput.(*devnetvm.SigLockedColoredOutput)
		if !ok {
			panic("failed to type cast SigLockedColoredOutput")
		}
		return typeCastedOutput.Address()
	default:
		panic("unexpected Output type")
	}
}

func makeTransaction(inputs devnetvm.Inputs, outputs devnetvm.Outputs, outputsByID map[utxo.OutputID]devnetvm.Output, walletsByAddress map[devnetvm.Address]wallet, genesisWallet ...wallet) *devnetvm.Transaction {
	txEssence := devnetvm.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, outputs)
	unlockBlocks := make([]devnetvm.UnlockBlock, len(txEssence.Inputs()))
	for i, input := range txEssence.Inputs() {
		w := wallet{}
		if genesisWallet != nil {
			w = genesisWallet[0]
		} else {
			w = walletsByAddress[addressFromInput(input, outputsByID)]
		}
		unlockBlocks[i] = devnetvm.NewSignatureUnlockBlock(w.sign(txEssence))
	}
	return devnetvm.NewTransaction(txEssence, unlockBlocks)
}

func selectIndex(transaction *devnetvm.Transaction, w wallet) (index uint16) {
	for i, output := range transaction.Essence().Outputs() {
		if w.address == output.(*devnetvm.SigLockedSingleOutput).Address() {
			return uint16(i)
		}
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	aMana               = 1.0
	totalAMana          = 1000.0
	testMaxBuffer       = 10000
	testRate            = time.Second / 5000
	tscThreshold        = 5 * time.Minute
	selfLocalIdentity   = identity.GenerateLocalIdentity()
	selfNode            = identity.New(selfLocalIdentity.PublicKey())
	peerNode            = identity.GenerateIdentity()
	testSchedulerParams = SchedulerParams{
		MaxBufferSize:                   testMaxBuffer,
		Rate:                            testRate,
		AccessManaMapRetrieverFunc:      mockAccessManaMapRetriever,
		TotalAccessManaRetrieveFunc:     mockTotalAccessManaRetriever,
		ConfirmedBlockScheduleThreshold: time.Minute,
	}
)

// mockAccessManaMapRetriever returns mocked access mana map.
func mockAccessManaMapRetriever() map[identity.ID]float64 {
	return map[identity.ID]float64{
		peerNode.ID(): aMana,
		selfNode.ID(): aMana,
	}
}

// mockAccessManaRetriever returns mocked access mana value for a node.
func mockAccessManaRetriever(id identity.ID) float64 {
	if id == peerNode.ID() || id == selfNode.ID() {
		return aMana
	}
	return 0
}

// mockTotalAccessManaRetriever returns mocked total access mana value.
func mockTotalAccessManaRetriever() float64 {
	return totalAMana
}

// NewTestTangle returns a Tangle instance with a testing schedulerConfig.
func NewTestTangle(options ...Option) *Tangle {
	cacheTimeProvider := database.NewCacheTimeProvider(0)

	options = append(options, SchedulerConfig(testSchedulerParams), CacheTimeProvider(cacheTimeProvider), TimeSinceConfirmationThreshold(tscThreshold))
	options = append(options, CommitmentFunc(func() (*epoch.ECRecord, epoch.Index, error) {
		return epoch.NewECRecord(0), 0, nil
	}))

	t := New(options...)
	t.ConfirmationOracle = &MockConfirmationOracle{}
	if t.WeightProvider == nil {
		t.WeightProvider = &MockWeightProvider{}
	}

	t.Events.Error.Hook(event.NewClosure(func(e error) {
		fmt.Println(e)
	}))

	return t
}

// MockConfirmationOracle is a mock of a ConfirmationOracle.
type MockConfirmationOracle struct {
	sync.RWMutex
}

// FirstUnconfirmedMarkerIndex mocks its interface function.
func (m *MockConfirmationOracle) FirstUnconfirmedMarkerIndex(sequenceID markers.SequenceID) (unconfirmedMarkerIndex markers.Index) {
	return 0
}

// IsMarkerConfirmed mocks its interface function.
func (m *MockConfirmationOracle) IsMarkerConfirmed(markers.Marker) bool {
	// We do not use the optimization in the AW manager via map for tests. Thus, in the test it always needs to start checking from the
	// beginning of the sequence for all markers.
	return false
}

// IsBlockConfirmed mocks its interface function.
func (m *MockConfirmationOracle) IsBlockConfirmed(blkID BlockID) bool {
	return false
}

// IsConflictConfirmed mocks its interface function.
func (m *MockConfirmationOracle) IsConflictConfirmed(conflictID utxo.TransactionID) bool {
	return false
}

// IsTransactionConfirmed mocks its interface function.
func (m *MockConfirmationOracle) IsTransactionConfirmed(transactionID utxo.TransactionID) bool {
	return transactionID == utxo.EmptyTransactionID
}

// IsOutputConfirmed mocks its interface function.
func (m *MockConfirmationOracle) IsOutputConfirmed(outputID utxo.OutputID) bool {
	return false
}

// Events mocks its interface function.
func (m *MockConfirmationOracle) Events() *ConfirmationEvents {
	return NewConfirmationEvents()
}

// MockWeightProvider is a mock of a WeightProvider.
type MockWeightProvider struct{}

func (m *MockWeightProvider) SnapshotEpochActivity(ei epoch.Index) (epochActivity epoch.SnapshotEpochActivity) {
	return nil
}

// LoadActiveNodes mocks its interface function.
func (m *MockWeightProvider) LoadActiveNodes(loadedActiveNodes epoch.SnapshotEpochActivity) {
}

// Update mocks its interface function.
func (m *MockWeightProvider) Update(ei epoch.Index, nodeID identity.ID) {
}

// Remove mocks its interface function.
func (m *MockWeightProvider) Remove(ei epoch.Index, nodeID identity.ID, count uint64) (removed bool) {
	return true
}

// Weight mocks its interface function.
func (m *MockWeightProvider) Weight(block *Block) (weight, totalWeight float64) {
	return 1, 1
}

// WeightsOfRelevantVoters mocks its interface function.
func (m *MockWeightProvider) WeightsOfRelevantVoters() (weights map[identity.ID]float64, totalWeight float64) {
	return
}

// Shutdown mocks its interface function.
func (m *MockWeightProvider) Shutdown() {
}

// SimpleMockOnTangleVoting is mock of OTV mechanism.
type SimpleMockOnTangleVoting struct {
	likedConflictMember map[utxo.TransactionID]LikedConflictMembers
}

// LikedConflictMembers is a struct that holds information about which Conflict is the liked one out of a set of
// ConflictMembers.
type LikedConflictMembers struct {
	likedConflict   utxo.TransactionID
	conflictMembers utxo.TransactionIDs
}

// LikedConflictMember returns conflicts that are liked instead of a disliked conflict as predefined.
func (o *SimpleMockOnTangleVoting) LikedConflictMember(conflictID utxo.TransactionID) (likedConflictID utxo.TransactionID, conflictMembers utxo.TransactionIDs) {
	likedConflictMembers := o.likedConflictMember[conflictID]
	innerConflictMembers := likedConflictMembers.conflictMembers.Clone()
	innerConflictMembers.Delete(conflictID)

	return likedConflictMembers.likedConflict, innerConflictMembers
}

// ConflictLiked returns whether the conflict is the winner across all conflict sets (it is in the liked reality).
func (o *SimpleMockOnTangleVoting) ConflictLiked(conflictID utxo.TransactionID) (conflictLiked bool) {
	likedConflictMembers, ok := o.likedConflictMember[conflictID]
	if !ok {
		return false
	}
	return likedConflictMembers.conflictMembers.Has(conflictID)
}

func emptyLikeReferences(payload payload.Payload, parents BlockIDs, _ time.Time) (references ParentBlockIDs, err error) {
	return emptyLikeReferencesFromStrongParents(parents), nil
}

func emptyLikeReferencesFromStrongParents(parents BlockIDs) (references ParentBlockIDs) {
	return NewParentBlockIDs().AddAll(StrongParentType, parents)
}

// EventMock acts as a container for event mocks.
type EventMock struct {
	mock.Mock
	expectedEvents uint64
	calledEvents   uint64
	test           *testing.T

	attached []struct {
		*event.Event[*BlockProcessedEvent]
		*event.Closure[*BlockProcessedEvent]
	}
}

// NewEventMock creates a new EventMock.
func NewEventMock(t *testing.T, approvalWeightManager *ApprovalWeightManager) *EventMock {
	e := &EventMock{
		test: t,
	}

	// attach all events
	approvalWeightManager.Events.ConflictWeightChanged.Hook(event.NewClosure(e.ConflictWeightChanged))
	approvalWeightManager.Events.MarkerWeightChanged.Hook(event.NewClosure(e.MarkerWeightChanged))
	approvalWeightManager.Events.BlockProcessed.Hook(event.NewClosure(e.BlockProcessed))

	// assure that all available events are mocked
	numEvents := reflect.ValueOf(approvalWeightManager.Events).Elem().NumField()
	assert.Equalf(t, len(e.attached)+3, numEvents, "not all events in ApprovalWeightManager.Events have been attached")

	return e
}

// DetachAll detaches all event handlers.
func (e *EventMock) DetachAll() {
	for _, a := range e.attached {
		a.Event.Detach(a.Closure)
	}
}

// Expect is a proxy for Mock.On() but keeping track of num of calls.
func (e *EventMock) Expect(eventName string, arguments ...interface{}) {
	e.On(eventName, arguments...)
	atomic.AddUint64(&e.expectedEvents, 1)
}

// AssertExpectations asserts expectations.
func (e *EventMock) AssertExpectations(t mock.TestingT) bool {
	calledEvents := atomic.LoadUint64(&e.calledEvents)
	expectedEvents := atomic.LoadUint64(&e.expectedEvents)
	if calledEvents != expectedEvents {
		t.Errorf("number of called (%d) events is not equal to number of expected events (%d)", calledEvents, expectedEvents)
		return false
	}

	defer func() {
		e.Calls = make([]mock.Call, 0)
		e.ExpectedCalls = make([]*mock.Call, 0)
		e.expectedEvents = 0
		e.calledEvents = 0
	}()

	return e.Mock.AssertExpectations(t)
}

// ConflictWeightChanged is the mocked ConflictWeightChanged function.
func (e *EventMock) ConflictWeightChanged(event *ConflictWeightChangedEvent) {
	e.Called(event.ConflictID, event.Weight)

	atomic.AddUint64(&e.calledEvents, 1)
}

// MarkerWeightChanged is the mocked MarkerWeightChanged function.
func (e *EventMock) MarkerWeightChanged(event *MarkerWeightChangedEvent) {
	e.Called(event.Marker, event.Weight)

	atomic.AddUint64(&e.calledEvents, 1)
}

// BlockProcessed is the mocked BlockProcessed function.
func (e *EventMock) BlockProcessed(event *BlockProcessedEvent) {
	e.Called(event.BlockID)

	atomic.AddUint64(&e.calledEvents, 1)
}

// loadSnapshotToLedger loads a snapshot of the Ledger from the given snapshot.
func loadSnapshotToLedger(l *ledger.Ledger, s *ledger.Snapshot) {
	l.LoadOutputWithMetadatas(s.OutputsWithMetadata)
	for _, diffs := range s.EpochDiffs {
		err := l.LoadEpochDiff(diffs)
		if err != nil {
			panic("Failed to load epochDiffs from snapshot")
		}
	}
}
