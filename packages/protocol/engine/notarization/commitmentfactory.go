package notarization

import (
	"context"

	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/objectstorage"
	"github.com/iotaledger/hive.go/core/kvstore"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
)

// region CommitmentFactory ///////////////////////////////////////////////////////////////////////////////////////

// CommitmentFactory manages epoch commitmentTrees.
type CommitmentFactory struct {
	storage *EpochCommitmentStorage

	// stateRootTree stores the state tree at the LastCommittedEpoch.
	stateRootTree *smt.SparseMerkleTree
	// manaRootTree stores the mana tree at the LastCommittedEpoch.
	manaRootTree *smt.SparseMerkleTree

	// snapshotDepth defines how far back the ledgerstate is kept with respect to the latest committed epoch.
	snapshotDepth int
}

// NewCommitmentFactory returns a new commitment factory.
func NewCommitmentFactory(store kvstore.KVStore, snapshotDepth int) *CommitmentFactory {
	epochCommitmentStorage := newEpochCommitmentStorage(WithStore(store))

	stateRootTreeNodeStore := objectstorage.NewStoreWithRealm(epochCommitmentStorage.baseStore, database.PrefixNotarization, prefixStateTreeNodes)
	stateRootTreeValueStore := objectstorage.NewStoreWithRealm(epochCommitmentStorage.baseStore, database.PrefixNotarization, prefixStateTreeValues)

	manaRootTreeNodeStore := objectstorage.NewStoreWithRealm(epochCommitmentStorage.baseStore, database.PrefixNotarization, prefixManaTreeNodes)
	manaRootTreeValueStore := objectstorage.NewStoreWithRealm(epochCommitmentStorage.baseStore, database.PrefixNotarization, prefixManaTreeValues)

	return &CommitmentFactory{
		storage:               epochCommitmentStorage,
		snapshotDepth:         snapshotDepth,
		stateRootTree:         smt.NewSparseMerkleTree(stateRootTreeNodeStore, stateRootTreeValueStore, lo.PanicOnErr(blake2b.New256(nil))),
		manaRootTree:          smt.NewSparseMerkleTree(manaRootTreeNodeStore, manaRootTreeValueStore, lo.PanicOnErr(blake2b.New256(nil))),
	}
}

// StateRoot returns the root of the state sparse merkle tree at the latest committed epoch.
func (f *CommitmentFactory) StateRoot() (stateRoot types.Identifier) {
	copy(stateRoot[:], f.stateRootTree.Root())

	return
}

// ManaRoot returns the root of the state sparse merkle tree at the latest committed epoch.
func (f *CommitmentFactory) ManaRoot() (manaRoot types.Identifier) {
	copy(manaRoot[:], f.manaRootTree.Root())

	return
}

// removeStateLeaf removes the output ID from the ledger sparse merkle tree.
func (f *CommitmentFactory) removeStateLeaf(outputID utxo.OutputID) error {
	exists, _ := f.stateRootTree.Has(outputID.Bytes())
	if exists {
		_, err := f.stateRootTree.Delete(outputID.Bytes())
		if err != nil {
			return errors.Wrap(err, "could not delete leaf from the state tree")
		}
	}
	return nil
}

// updateManaLeaf updates the mana balance in the mana sparse merkle tree.
func (f *CommitmentFactory) updateManaLeaf(outputWithMetadata *ledger.OutputWithMetadata, isCreated bool) (err error) {
	outputBalance, exists := outputWithMetadata.Output().(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA)
	if !exists {
		return nil
	}

	accountBytes := outputWithMetadata.ConsensusManaPledgeID().Bytes()

	var currentBalance uint64
	if balanceBytes, getLeafErr := f.manaRootTree.Get(accountBytes); getLeafErr != nil && len(balanceBytes) > 0 {
		_, decodeErr := serix.DefaultAPI.Decode(context.Background(), balanceBytes, &currentBalance, serix.WithValidation())
		if decodeErr != nil {
			return errors.Wrap(decodeErr, "could not decode mana leaf balance")
		}
	}

	if isCreated {
		currentBalance += outputBalance
	} else {
		currentBalance -= outputBalance
	}

	// remove leaf if mana is zero
	if currentBalance <= 0 {
		return lo.Return2(f.manaRootTree.Delete(accountBytes))
	}

	encodedBalanceBytes, encodeErr := serix.DefaultAPI.Encode(context.Background(), currentBalance, serix.WithValidation())
	if encodeErr != nil {
		return errors.Wrap(encodeErr, "could not encode mana leaf balance")
	}

	return lo.Return2(f.manaRootTree.Update(accountBytes, encodedBalanceBytes))
}


// ecRecord retrieves the epoch commitment.
func (f *CommitmentFactory) createCommitment(ei epoch.Index) (ecRecord *chainmanager.Commitment, err error) {
	ecRecord = f.loadECRecord(ei)
	if ecRecord != nil {
		return ecRecord, nil
	}
	// We never committed this epoch before, create and roll to a new epoch.
	// TODO: what if a node restarts and we have incomplete tr1ees?
	commitmentTrees, commitmentTreesErr := f.getCommitmentTrees(ei)
	if commitmentTreesErr != nil {
		return nil, errors.Wrapf(commitmentTreesErr, "cannot get commitment tree for epoch %d", ei)
	}
	roots, ecrErr := f.ECRandRoots(ei, commitmentTrees)
	if ecrErr != nil {
		return nil, ecrErr
	}
	prevECRecord, ecrRecordErr := f.createCommitment(ei - 1)
	if ecrRecordErr != nil {
		return nil, ecrRecordErr
	}

	newCommitment := commitment.New(ei, prevECRecord.ID(), roots.ID(), 0) // TODO: REPLACE CUMULATIVE WEIGHT

	// Store and return.
	f.storage.CachedECRecord(ei, func(ei epoch.Index) *chainmanager.Commitment {
		return chainmanager.NewCommitment(newCommitment.ID())
	}).Consume(func(e *chainmanager.Commitment) {
		ecRecord = e
		ecRecord.PublishCommitment(newCommitment)
		ecRecord.PublishRoots(roots)
	})

	return ecRecord, nil
}



func (f *CommitmentFactory) loadECRecord(ei epoch.Index) (ecRecord *chainmanager.Commitment) {
	f.storage.CachedECRecord(ei).Consume(func(record *chainmanager.Commitment) {
		ecRecord = record
	})
	return
}

// storeDiffUTXOs stores the diff UTXOs occurred on an epoch without removing UTXOs created and spent in the span of a
// single epoch. This is because, as UTXOs can be stored out-of-order, we cannot reliably remove intermediate UTXOs
// before an epoch is committable.
func (f *CommitmentFactory) storeDiffUTXOs(ei epoch.Index, spent, created []*ledger.OutputWithMetadata) {
	epochDiffStorage := f.storage.getEpochDiffStorage(ei)

	for _, spentOutputWithMetadata := range spent {
		cachedObj, stored := epochDiffStorage.spent.StoreIfAbsent(spentOutputWithMetadata)
		if !stored {
			continue
		}
		cachedObj.Release()
	}

	for _, createdOutputWithMetadata := range created {
		cachedObj, stored := epochDiffStorage.created.StoreIfAbsent(createdOutputWithMetadata)
		if !stored {
			continue
		}
		cachedObj.Release()
	}
}

func (f *CommitmentFactory) deleteDiffUTXOs(ei epoch.Index, spent, created []*ledger.OutputWithMetadata) {
	epochDiffStorage := f.storage.getEpochDiffStorage(ei)

	for _, spentOutputWithMetadata := range spent {
		epochDiffStorage.spent.Delete(spentOutputWithMetadata.ID().Bytes())
	}

	for _, createdOutputWithMetadata := range created {
		epochDiffStorage.created.Delete(createdOutputWithMetadata.ID().Bytes())
	}
}

// loadDiffUTXOs loads the diff UTXOs occurred on an epoch by removing UTXOs created and spent in the span of the same epoch,
// as by the time we load a diff we assume the epoch is being committed and cannot be altered anymore.
func (f *CommitmentFactory) loadDiffUTXOs(ei epoch.Index) (spent, created []*ledger.OutputWithMetadata) {
	epochDiffStorage := f.storage.getEpochDiffStorage(ei)

	spent = make([]*ledger.OutputWithMetadata, 0)
	epochDiffStorage.spent.ForEach(func(_ []byte, cachedOutputWithMetadata *objectstorage.CachedObject[*ledger.OutputWithMetadata]) bool {
		cachedOutputWithMetadata.Consume(func(outputWithMetadata *ledger.OutputWithMetadata) {
			// We remove spent and created UTXOs happened in the same epoch, as we assume that by the time we
			// load the epoch diff, the epoch is being committed and cannot be altered anymore.
			if epochDiffStorage.created.DeleteIfPresent(outputWithMetadata.ID().Bytes()) {
				epochDiffStorage.spent.Delete(outputWithMetadata.ID().Bytes())
				return
			}
			spent = append(spent, outputWithMetadata)
		})
		return true
	})

	created = make([]*ledger.OutputWithMetadata, 0)
	epochDiffStorage.created.ForEach(func(_ []byte, cachedOutputWithMetadata *objectstorage.CachedObject[*ledger.OutputWithMetadata]) bool {
		cachedOutputWithMetadata.Consume(func(outputWithMetadata *ledger.OutputWithMetadata) {
			created = append(created, outputWithMetadata)
		})
		return true
	})

	return
}

func (f *CommitmentFactory) loadLedgerState(consumer func(*ledger.OutputWithMetadata)) {
	f.storage.ledgerstateStorage.ForEach(func(_ []byte, cachedOutputWithMetadata *objectstorage.CachedObject[*ledger.OutputWithMetadata]) bool {
		cachedOutputWithMetadata.Consume(consumer)
		return true
	})

	return
}

// ECRandRoots creates a new commitment with the given ei, by advancing the corresponding data structures.
func (f *CommitmentFactory) ECRandRoots(ei epoch.Index, commitmentTrees *commitmentTrees) (roots *commitment.Roots, commitmentTreesErr error) {
	// We advance the StateRootTree to the next epoch.
	stateRoot, manaRoot, newStateRootsErr := f.newStateRoots(ei)
	if newStateRootsErr != nil {
		return nil, errors.Wrapf(newStateRootsErr, "cannot get state roots for epoch %d", ei)
	}

	// We advance the LedgerState to the next epoch.
	epochToCommit := ei - epoch.Index(f.snapshotDepth)
	if epochToCommit > 0 {
		f.commitLedgerState(epochToCommit)
	}

	return commitment.NewRoots(
		stateRoot,
		manaRoot,
		commitmentTrees.TangleRoot(),
		commitmentTrees.StateMutationRoot(),
		commitmentTrees.ActivityRoot(),
	), nil
}

// commitLedgerState commits the corresponding diff to the ledger state and drops it.
func (f *CommitmentFactory) commitLedgerState(ei epoch.Index) {
	spent, created := f.loadDiffUTXOs(ei)
	for _, spentOutputWithMetadata := range spent {
		f.storage.ledgerstateStorage.Delete(spentOutputWithMetadata.ID().Bytes())
	}

	for _, createdOutputWithMetadata := range created {
		f.storage.ledgerstateStorage.Store(createdOutputWithMetadata).Release()
	}

	f.storage.dropEpochDiffStorage(ei)

	return
}

func (f *CommitmentFactory) newStateRoots(ei epoch.Index) (stateRoot types.Identifier, manaRoot types.Identifier, err error) {
	// By the time we want the state root for a specific epoch, the diff should be complete and unalterable.
	spentOutputs, createdOutputs := f.loadDiffUTXOs(ei)

	// Insert  created UTXOs into the state tree.
	for _, created := range createdOutputs {
		if _, err = f.stateRootTree.Update(created.ID().Bytes(), created.ID().Bytes()); err != nil {
			return types.Identifier{}, types.Identifier{}, errors.Wrap(err, "could not insert the state leaf")
		}

		if err = f.updateManaLeaf(created, true); err != nil {
			return types.Identifier{}, types.Identifier{}, errors.Wrap(err, "could not insert the mana leaf")
		}
	}

	// Remove spent UTXOs from the state tree.
	for _, spent := range spentOutputs {
		if _, err = f.stateRootTree.Delete(spent.ID().Bytes()); err != nil {
			return types.Identifier{}, types.Identifier{}, errors.Wrap(err, "could not remove state leaf")
		}
		err = f.updateManaLeaf(spent, false)
		if err != nil {
			return types.Identifier{}, types.Identifier{}, errors.Wrap(err, "could not remove mana leaf")
		}
	}

	return f.StateRoot(), f.ManaRoot(), nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Commitment types /////////////////////////////////////////////////////////////////////////////////////////////

// commitmentTrees is a compressed form of all the information (blocks and confirmed value payloads) of an epoch.
type commitmentTrees struct {
	EI                epoch.Index
	tangleTree        *smt.SparseMerkleTree
	stateMutationTree *smt.SparseMerkleTree
	activityTree      *smt.SparseMerkleTree
}

func newCommitmentTrees(ei epoch.Index) *commitmentTrees {
	return &commitmentTrees{
		EI:                ei,
		tangleTree:        smt.NewSparseMerkleTree(mapdb.NewMapDB(), mapdb.NewMapDB(), lo.PanicOnErr(blake2b.New256(nil))),
		stateMutationTree: smt.NewSparseMerkleTree(mapdb.NewMapDB(), mapdb.NewMapDB(), lo.PanicOnErr(blake2b.New256(nil))),
		activityTree:      smt.NewSparseMerkleTree(mapdb.NewMapDB(), mapdb.NewMapDB(), lo.PanicOnErr(blake2b.New256(nil))),
	}
}

func (c *commitmentTrees) TangleRoot() (tangleRoot types.Identifier) {
	copy(tangleRoot[:], c.tangleTree.Root())

	return
}

func (c *commitmentTrees) StateMutationRoot() (stateMutationRoot types.Identifier) {
	copy(stateMutationRoot[:], c.stateMutationTree.Root())

	return
}

func (c *commitmentTrees) ActivityRoot() (activityRoot types.Identifier) {
	copy(activityRoot[:], c.activityTree.Root())

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
