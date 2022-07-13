package notarization

import (
	"context"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/hive.go/serix"

	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/kvstore"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/tangle"
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

// CommitmentTrees is a compressed form of all the information (messages and confirmed value payloads) of an epoch.
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
	tangle  *tangle.Tangle

	// stateRootTree stores the state tree at the LastCommittedEpoch.
	stateRootTree *smt.SparseMerkleTree
	// manaRootTree stores the mana tree at the LastCommittedEpoch + 1.
	manaRootTree *smt.SparseMerkleTree

	// snapshotDepth defines how far back the ledgerstate is kept with respect to the latest committed epoch.
	snapshotDepth int
}

// NewEpochCommitmentFactory returns a new commitment factory.
func NewEpochCommitmentFactory(store kvstore.KVStore, tangle *tangle.Tangle, snapshotDepth int) *EpochCommitmentFactory {
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

	root := make([]byte, 0)
	branch1 := make([]byte, 0)
	branch1a := make([]byte, 0)
	branch1b := make([]byte, 0)
	branch2 := make([]byte, 0)

	branch1aHashed := blake2b.Sum256(append(append(branch1a, epochRoots.tangleRoot[:]...), epochRoots.stateMutationRoot[:]...))
	branch1bHashed := blake2b.Sum256(append(append(branch1b, epochRoots.stateRoot[:]...), epochRoots.manaRoot[:]...))
	branch1Hashed := blake2b.Sum256(append(append(branch1, branch1aHashed[:]...), branch1bHashed[:]...))
	branch2Hashed := blake2b.Sum256(append(branch2, epochRoots.activityRoot[:]...))
	rootHashed := blake2b.Sum256(append(append(root, branch1Hashed[:]...), branch2Hashed[:]...))

	return epoch.NewMerkleRoot(rootHashed[:]), nil
}

// InsertStateLeaf inserts the outputID to the state sparse merkle tree.
func (f *EpochCommitmentFactory) insertStateLeaf(outputID utxo.OutputID) error {
	_, err := f.stateRootTree.Update(outputID.Bytes(), outputID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state tree")
	}
	return nil
}

// RemoveStateLeaf removes the output ID from the ledger sparse merkle tree.
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

// UpdateManaLeaf updates the mana balance in the mana sparse merkle tree.
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
		if _, deleteLeafErr := f.manaRootTree.Delete(accountBytes); deleteLeafErr != nil {
			return errors.Wrap(deleteLeafErr, "could not delete leaf from mana tree")
		}
		return nil
	}

	encodedBalanceBytes, encodeErr := serix.DefaultAPI.Encode(context.Background(), currentBalance, serix.WithValidation())
	if encodeErr != nil {
		return errors.Wrap(encodeErr, "could not encode mana leaf balance")
	}

	if _, updateLeafErr := f.manaRootTree.Update(accountBytes, encodedBalanceBytes); updateLeafErr != nil {
		return errors.Wrap(updateLeafErr, "could not update mana tree leaf")
	}

	return nil
}

// InsertStateMutationLeaf inserts the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) insertStateMutationLeaf(ei epoch.Index, txID utxo.TransactionID) error {
	commitment, err := f.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting state mutation leaf")
	}
	_, err = commitment.stateMutationTree.Update(txID.Bytes(), txID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state mutation tree")
	}
	return nil
}

// RemoveStateMutationLeaf deletes the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) removeStateMutationLeaf(ei epoch.Index, txID utxo.TransactionID) error {
	commitment, err := f.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while deleting state mutation leaf")
	}
	_, err = commitment.stateMutationTree.Delete(txID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not delete leaf from the state mutation tree")
	}
	return nil
}

// InsertTangleLeaf inserts msg to the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) insertTangleLeaf(ei epoch.Index, msgID tangle.MessageID) error {
	commitment, err := f.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting tangle leaf")
	}
	_, err = commitment.tangleTree.Update(msgID.Bytes(), msgID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the tangle tree")
	}
	return nil
}

// RemoveTangleLeaf removes the message ID from the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) removeTangleLeaf(ei epoch.Index, msgID tangle.MessageID) error {
	commitment, err := f.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while deleting tangle leaf")
	}
	exists, _ := commitment.tangleTree.Has(msgID.Bytes())
	if exists {
		_, err2 := commitment.tangleTree.Delete(msgID.Bytes())
		if err2 != nil {
			return errors.Wrap(err, "could not delete leaf from the tangle tree")
		}
	}
	return nil
}

// insertActivityLeaf inserts nodeID to the Activity sparse merkle tree along with a counter corresponding to
// the number of accepted blocks issued by this node.
func (f *EpochCommitmentFactory) insertActivityLeaf(ei epoch.Index, nodeID identity.ID) error {
	commitment, err := f.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting tangle leaf")
	}

	var blocksAccepted uint64
	exists, _ := commitment.activityTree.Has(nodeID.Bytes())
	if exists {
		if blocksAcceptedBytes, activeErr := commitment.activityTree.Get(nodeID.Bytes()); activeErr == nil {
			_, decodeErr := serix.DefaultAPI.Decode(context.Background(), blocksAcceptedBytes, &blocksAccepted, serix.WithValidation())
			if decodeErr != nil {
				return errors.Wrap(decodeErr, "could not decode accepted blocks count for activity tree")
			}
		}
	}
	blocksAccepted++
	encodedAcceptedBytes, encodeErr := serix.DefaultAPI.Encode(context.Background(), blocksAccepted, serix.WithValidation())
	if encodeErr != nil {
		return errors.Wrap(encodeErr, "could not encode active count for activity leaf ")
	}
	_, updateErr := commitment.tangleTree.Update(nodeID.Bytes(), encodedAcceptedBytes)
	if updateErr != nil {
		return errors.Wrap(err, "could not insert leaf to the activity tree")
	}

	return nil
}

// removeActivityLeaf removes the nodeID from the Activity sparse merkle tree
// if node had only one block accepted this far for this epoch.
func (f *EpochCommitmentFactory) removeActivityLeaf(ei epoch.Index, nodeID identity.ID) (removed bool, err error) {
	commitment, err := f.getCommitmentTrees(ei)
	if err != nil {
		return false, errors.Wrap(err, "could not get commitment while deleting tangle leaf")
	}
	exists, _ := commitment.activityTree.Has(nodeID.Bytes())
	if exists {
		var blocksAccepted uint64
		if blocksAcceptedBytes, activeErr := commitment.activityTree.Get(nodeID.Bytes()); activeErr == nil {
			_, decodeErr := serix.DefaultAPI.Decode(context.Background(), blocksAcceptedBytes, &blocksAccepted, serix.WithValidation())
			if decodeErr != nil {
				return false, errors.Wrap(decodeErr, "could not decode accepted blocks count for activity tree")
			}
			if blocksAccepted == 1 {
				_, err2 := commitment.tangleTree.Delete(nodeID.Bytes())
				if err2 != nil {
					return false, errors.Wrap(err, "could not delete leaf from the tangle tree")
				}
				return true, nil
			}
			blocksAccepted--
			encodedAcceptedBytes, encodeErr := serix.DefaultAPI.Encode(context.Background(), blocksAccepted, serix.WithValidation())
			if encodeErr != nil {
				return false, errors.Wrap(encodeErr, "could not encode active count for activity leaf ")
			}
			_, updateErr := commitment.tangleTree.Update(nodeID.Bytes(), encodedAcceptedBytes)
			if updateErr != nil {
				return false, errors.Wrap(err, "could not insert leaf to the activity tree")
			}
		}
	}
	return false, nil
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

// NewCommitment returns an empty commitment for the epoch.
func (f *EpochCommitmentFactory) newCommitmentTrees(ei epoch.Index) *CommitmentTrees {
	// Volatile storage for small trees
	db, _ := database.NewMemDB()
	messageIDStore := db.NewStore()
	messageValueStore := db.NewStore()
	stateMutationIDStore := db.NewStore()
	stateMutationValueStore := db.NewStore()
	activityValueStore := db.NewStore()
	activityIDStore := db.NewStore()

	commitmentTrees := &CommitmentTrees{
		EI:                ei,
		tangleTree:        smt.NewSparseMerkleTree(messageIDStore, messageValueStore, lo.PanicOnErr(blake2b.New256(nil))),
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
		err = f.insertStateLeaf(created.ID())
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
		err = f.removeStateLeaf(spent.ID())
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
