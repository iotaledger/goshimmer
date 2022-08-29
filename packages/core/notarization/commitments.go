package notarization

import (
	"context"

	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/byteutils"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/database"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/objectstorage"
	"github.com/iotaledger/hive.go/core/kvstore"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

// region Commitment types ////////////////////////////////////////////////////////////////////////////////////////////

// CommitmentRoots contains roots of trees of an epoch.
type CommitmentRoots struct {
	EI                epoch.Index
	tangleRoot        epoch.MerkleRoot
	stateMutationRoot epoch.MerkleRoot
	stateRoot         epoch.MerkleRoot
	manaRoot          epoch.MerkleRoot
	activityRoot      epoch.MerkleRoot
}

// CommitmentTrees is a compressed form of all the information (blocks and confirmed value payloads) of an epoch.
type CommitmentTrees struct {
	EI                epoch.Index
	tangleTree        *smt.SparseMerkleTree
	stateMutationTree *smt.SparseMerkleTree
	activityTree      *smt.SparseMerkleTree
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EpochCommitmentFactory ///////////////////////////////////////////////////////////////////////////////////////

// EpochCommitmentFactory manages epoch commitmentTrees.
type EpochCommitmentFactory struct {
	commitmentTrees map[epoch.Index]*CommitmentTrees

	storage *EpochCommitmentStorage
	tangle  *tangleold.Tangle

	// stateRootTree stores the state tree at the LastCommittedEpoch.
	stateRootTree *smt.SparseMerkleTree
	// manaRootTree stores the mana tree at the LastCommittedEpoch.
	manaRootTree *smt.SparseMerkleTree

	// snapshotDepth defines how far back the ledgerstate is kept with respect to the latest committed epoch.
	snapshotDepth int
}

// NewEpochCommitmentFactory returns a new commitment factory.
func NewEpochCommitmentFactory(store kvstore.KVStore, tangle *tangleold.Tangle, snapshotDepth int) *EpochCommitmentFactory {
	epochCommitmentStorage := newEpochCommitmentStorage(WithStore(store))

	stateRootTreeNodeStore := objectstorage.NewStoreWithRealm(epochCommitmentStorage.baseStore, database.PrefixNotarization, prefixStateTreeNodes)
	stateRootTreeValueStore := objectstorage.NewStoreWithRealm(epochCommitmentStorage.baseStore, database.PrefixNotarization, prefixStateTreeValues)

	manaRootTreeNodeStore := objectstorage.NewStoreWithRealm(epochCommitmentStorage.baseStore, database.PrefixNotarization, prefixManaTreeNodes)
	manaRootTreeValueStore := objectstorage.NewStoreWithRealm(epochCommitmentStorage.baseStore, database.PrefixNotarization, prefixManaTreeValues)

	return &EpochCommitmentFactory{
		commitmentTrees: make(map[epoch.Index]*CommitmentTrees),
		storage:         epochCommitmentStorage,
		tangle:          tangle,
		snapshotDepth:   snapshotDepth,
		stateRootTree:   smt.NewSparseMerkleTree(stateRootTreeNodeStore, stateRootTreeValueStore, lo.PanicOnErr(blake2b.New256(nil))),
		manaRootTree:    smt.NewSparseMerkleTree(manaRootTreeNodeStore, manaRootTreeValueStore, lo.PanicOnErr(blake2b.New256(nil))),
	}
}

// StateRoot returns the root of the state sparse merkle tree at the latest committed epoch.
func (f *EpochCommitmentFactory) StateRoot() []byte {
	return f.stateRootTree.Root()
}

// ManaRoot returns the root of the state sparse merkle tree at the latest committed epoch.
func (f *EpochCommitmentFactory) ManaRoot() []byte {
	return f.manaRootTree.Root()
}

// ECR retrieves the epoch commitment root.
func (f *EpochCommitmentFactory) ECR(ei epoch.Index) (ecr epoch.ECR, err error) {
	epochRoots, err := f.newEpochRoots(ei)
	if err != nil {
		return epoch.MerkleRoot{}, errors.Wrap(err, "ECR could not be created")
	}

	branch1aHashed := blake2b.Sum256(byteutils.ConcatBytes(epochRoots.tangleRoot[:], epochRoots.stateMutationRoot[:]))
	branch1bHashed := blake2b.Sum256(byteutils.ConcatBytes(epochRoots.stateRoot[:], epochRoots.manaRoot[:]))
	branch1Hashed := blake2b.Sum256(byteutils.ConcatBytes(branch1aHashed[:], branch1bHashed[:]))
	branch2Hashed := blake2b.Sum256(epochRoots.activityRoot[:])
	rootHashed := blake2b.Sum256(byteutils.ConcatBytes(branch1Hashed[:], branch2Hashed[:]))

	return epoch.NewMerkleRoot(rootHashed[:]), nil
}

// removeStateLeaf removes the output ID from the ledger sparse merkle tree.
func (f *EpochCommitmentFactory) removeStateLeaf(outputID utxo.OutputID) error {
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
func (f *EpochCommitmentFactory) updateManaLeaf(outputWithMetadata *ledger.OutputWithMetadata, isCreated bool) (err error) {
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
		return removeLeaf(f.manaRootTree, accountBytes)
	}

	encodedBalanceBytes, encodeErr := serix.DefaultAPI.Encode(context.Background(), currentBalance, serix.WithValidation())
	if encodeErr != nil {
		return errors.Wrap(encodeErr, "could not encode mana leaf balance")
	}

	return insertLeaf(f.manaRootTree, accountBytes, encodedBalanceBytes)
}

// insertStateMutationLeaf inserts the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) insertStateMutationLeaf(ei epoch.Index, txID utxo.TransactionID) error {
	commitment, err := f.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting state mutation leaf")
	}
	return insertLeaf(commitment.stateMutationTree, txID.Bytes(), txID.Bytes())
}

// removeStateMutationLeaf deletes the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) removeStateMutationLeaf(ei epoch.Index, txID utxo.TransactionID) error {
	commitment, err := f.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while deleting state mutation leaf")
	}
	return removeLeaf(commitment.stateMutationTree, txID.Bytes())
}

// insertTangleLeaf inserts blk to the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) insertTangleLeaf(ei epoch.Index, blkID tangleold.BlockID) error {
	commitment, err := f.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting tangle leaf")
	}
	return insertLeaf(commitment.tangleTree, blkID.Bytes(), blkID.Bytes())
}

// removeTangleLeaf removes the block ID from the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) removeTangleLeaf(ei epoch.Index, blkID tangleold.BlockID) error {
	commitment, err := f.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while deleting tangle leaf")
	}
	return removeLeaf(commitment.tangleTree, blkID.Bytes())
}

// insertActivityLeaf inserts nodeID to the Activity sparse merkle tree.
func (f *EpochCommitmentFactory) insertActivityLeaf(ei epoch.Index, nodeID identity.ID, acceptedInc ...uint64) error {
	commitment, err := f.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting activity leaf")
	}
	return insertLeaf(commitment.activityTree, nodeID.Bytes(), nodeID.Bytes())
}

// removeActivityLeaf removes the nodeID from the Activity sparse merkle tree.
func (f *EpochCommitmentFactory) removeActivityLeaf(ei epoch.Index, nodeID identity.ID) error {
	commitment, err := f.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while deleting activity leaf")
	}
	return removeLeaf(commitment.activityTree, nodeID.Bytes())
}

// ecRecord retrieves the epoch commitment.
func (f *EpochCommitmentFactory) ecRecord(ei epoch.Index) (ecRecord *epoch.ECRecord, err error) {
	ecRecord = f.loadECRecord(ei)
	if ecRecord != nil {
		return ecRecord, nil
	}
	// We never committed this epoch before, create and roll to a new epoch.
	ecr, ecrErr := f.ECR(ei)
	if ecrErr != nil {
		return nil, ecrErr
	}
	prevECRecord, ecrRecordErr := f.ecRecord(ei - 1)
	if ecrRecordErr != nil {
		return nil, ecrRecordErr
	}

	// Store and return.
	f.storage.CachedECRecord(ei, epoch.NewECRecord).Consume(func(e *epoch.ECRecord) {
		e.SetECR(ecr)
		e.SetPrevEC(EC(prevECRecord))
		ecRecord = e
	})

	return ecRecord, nil
}

func (f *EpochCommitmentFactory) loadECRecord(ei epoch.Index) (ecRecord *epoch.ECRecord) {
	f.storage.CachedECRecord(ei).Consume(func(record *epoch.ECRecord) {
		ecRecord = epoch.NewECRecord(ei)
		ecRecord.SetECR(record.ECR())
		ecRecord.SetPrevEC(record.PrevEC())
	})
	return
}

// storeDiffUTXOs stores the diff UTXOs occurred on an epoch without removing UTXOs created and spent in the span of a
// single epoch. This is because, as UTXOs can be stored out-of-order, we cannot reliably remove intermediate UTXOs
// before an epoch is committable.
func (f *EpochCommitmentFactory) storeDiffUTXOs(ei epoch.Index, spent, created []*ledger.OutputWithMetadata) {
	epochDiffStorage := f.storage.getEpochDiffStorage(ei)

	for _, spentOutputWithMetadata := range spent {
		epochDiffStorage.spent.Store(spentOutputWithMetadata).Release()
	}

	for _, createdOutputWithMetadata := range created {
		epochDiffStorage.created.Store(createdOutputWithMetadata).Release()
	}
}

func (f *EpochCommitmentFactory) deleteDiffUTXOs(ei epoch.Index, spent, created []*ledger.OutputWithMetadata) {
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
func (f *EpochCommitmentFactory) loadDiffUTXOs(ei epoch.Index) (spent, created []*ledger.OutputWithMetadata) {
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

func (f *EpochCommitmentFactory) loadLedgerState(consumer func(*ledger.OutputWithMetadata)) {
	f.storage.ledgerstateStorage.ForEach(func(_ []byte, cachedOutputWithMetadata *objectstorage.CachedObject[*ledger.OutputWithMetadata]) bool {
		cachedOutputWithMetadata.Consume(consumer)
		return true
	})

	return
}

// NewCommitment returns an empty commitment for the epoch.
func (f *EpochCommitmentFactory) newCommitmentTrees(ei epoch.Index) *CommitmentTrees {
	// Volatile storage for small trees
	db, _ := database.NewMemDB("")
	blockIDStore := db.NewStore()
	blockValueStore := db.NewStore()
	stateMutationIDStore := db.NewStore()
	stateMutationValueStore := db.NewStore()
	activityValueStore := db.NewStore()
	activityIDStore := db.NewStore()

	commitmentTrees := &CommitmentTrees{
		EI:                ei,
		tangleTree:        smt.NewSparseMerkleTree(blockIDStore, blockValueStore, lo.PanicOnErr(blake2b.New256(nil))),
		stateMutationTree: smt.NewSparseMerkleTree(stateMutationIDStore, stateMutationValueStore, lo.PanicOnErr(blake2b.New256(nil))),
		activityTree:      smt.NewSparseMerkleTree(activityIDStore, activityValueStore, lo.PanicOnErr(blake2b.New256(nil))),
	}

	return commitmentTrees
}

// newEpochRoots creates a new commitment with the given ei, by advancing the corresponding data structures.
func (f *EpochCommitmentFactory) newEpochRoots(ei epoch.Index) (commitmentRoots *CommitmentRoots, commitmentTreesErr error) {
	// TODO: what if a node restarts and we have incomplete trees?
	commitmentTrees, commitmentTreesErr := f.getCommitmentTrees(ei)
	if commitmentTreesErr != nil {
		return nil, errors.Wrapf(commitmentTreesErr, "cannot get commitment tree for epoch %d", ei)
	}

	// We advance the StateRootTree to the next epoch.
	stateRoot, manaRoot, newStateRootsErr := f.newStateRoots(ei)
	if newStateRootsErr != nil {
		return nil, errors.Wrapf(newStateRootsErr, "cannot get state roots for epoch %d", ei)
	}

	// We advance the LedgerState to the next epoch.
	f.commitLedgerState(ei - epoch.Index(f.snapshotDepth))

	commitmentRoots = &CommitmentRoots{
		EI:                ei,
		stateRoot:         epoch.NewMerkleRoot(stateRoot),
		manaRoot:          epoch.NewMerkleRoot(manaRoot),
		tangleRoot:        epoch.NewMerkleRoot(commitmentTrees.tangleTree.Root()),
		stateMutationRoot: epoch.NewMerkleRoot(commitmentTrees.stateMutationTree.Root()),
	}

	// We are never going to use this epoch's commitment trees again.
	delete(f.commitmentTrees, ei)

	return commitmentRoots, nil
}

// commitLedgerState commits the corresponding diff to the ledger state and drops it.
func (f *EpochCommitmentFactory) commitLedgerState(ei epoch.Index) {
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

func (f *EpochCommitmentFactory) getCommitmentTrees(ei epoch.Index) (commitmentTrees *CommitmentTrees, err error) {
	lastCommittedEpoch, lastCommittedEpochErr := f.storage.latestCommittableEpochIndex()
	if lastCommittedEpochErr != nil {
		return nil, errors.Wrap(lastCommittedEpochErr, "cannot get last committed epoch")
	}
	if ei <= lastCommittedEpoch {
		return nil, errors.Errorf("cannot get commitment trees for epoch %d, because it is already committed", ei)
	}
	commitmentTrees, ok := f.commitmentTrees[ei]
	if !ok {
		commitmentTrees = f.newCommitmentTrees(ei)
		f.commitmentTrees[ei] = commitmentTrees
	}
	return
}

func (f *EpochCommitmentFactory) newStateRoots(ei epoch.Index) (stateRoot []byte, manaRoot []byte, err error) {
	// By the time we want the state root for a specific epoch, the diff should be complete and unalterable.
	spentOutputs, createdOutputs := f.loadDiffUTXOs(ei)

	// Insert  created UTXOs into the state tree.
	for _, created := range createdOutputs {
		err = insertLeaf(f.stateRootTree, created.ID().Bytes(), created.ID().Bytes())
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not insert the state leaf")
		}
		err = f.updateManaLeaf(created, true)
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not insert the mana leaf")
		}
	}

	// Remove spent UTXOs from the state tree.
	for _, spent := range spentOutputs {
		err = removeLeaf(f.stateRootTree, spent.ID().Bytes())
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not remove state leaf")
		}
		err = f.updateManaLeaf(spent, false)
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not remove mana leaf")
		}
	}

	return f.StateRoot(), f.ManaRoot(), nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region extra functions //////////////////////////////////////////////////////////////////////////////////////////////

// EC calculates the epoch commitment hash from the given ECRecord.
func EC(ecRecord *epoch.ECRecord) (ec epoch.EC) {
	concatenated := make([]byte, 0)
	concatenated = append(concatenated, ecRecord.EI().Bytes()...)
	concatenated = append(concatenated, ecRecord.ECR().Bytes()...)
	concatenated = append(concatenated, ecRecord.PrevEC().Bytes()...)

	ecHash := blake2b.Sum256(concatenated)

	return epoch.NewMerkleRoot(ecHash[:])
}

// insertLeaf inserts the outputID to the provided sparse merkle tree.
func insertLeaf(tree *smt.SparseMerkleTree, keyBytes, valueBytes []byte) error {
	_, err := tree.Update(keyBytes, valueBytes)
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the tree")
	}
	return nil
}

// removeLeaf inserts the outputID to the provided sparse merkle tree.
func removeLeaf(tree *smt.SparseMerkleTree, leaf []byte) error {
	exists, _ := tree.Has(leaf)
	if exists {
		_, err := tree.Delete(leaf)
		if err != nil {
			return errors.Wrap(err, "could not delete leaf from the tree")
		}
	}
	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
