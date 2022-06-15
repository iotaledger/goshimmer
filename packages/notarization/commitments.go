package notarization

import (
	"fmt"
	"github.com/iotaledger/hive.go/identity"
	"hash"
	"sync"

	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// CommitmentRoots contains roots of trees of an epoch.
type CommitmentRoots struct {
	EI                epoch.Index
	tangleRoot        epoch.MerkleRoot
	stateMutationRoot epoch.MerkleRoot
	stateRoot         epoch.MerkleRoot
	manaRoot          epoch.MerkleRoot
}

// CommitmentTrees is a compressed form of all the information (messages and confirmed value payloads) of an epoch.
type CommitmentTrees struct {
	EI                epoch.Index
	tangleTree        *smt.SparseMerkleTree
	stateMutationTree *smt.SparseMerkleTree
}

// TangleRoot returns the root of the tangle sparse merkle tree.
func (e *CommitmentTrees) TangleRoot() []byte {
	return e.tangleTree.Root()
}

// StateMutationRoot returns the root of the state mutation sparse merkle tree.
func (e *CommitmentTrees) StateMutationRoot() []byte {
	return e.stateMutationTree.Root()
}

// EpochCommitmentFactory manages epoch commitmentTrees.
type EpochCommitmentFactory struct {
	commitmentTrees  map[epoch.Index]*CommitmentTrees
	commitmentsMutex sync.RWMutex

	storage *EpochCommitmentStorage
	tangle  *tangle.Tangle
	hasher  hash.Hash

	// stateRootTree stores the state tree at the LastCommittedEpoch.
	stateRootTree *smt.SparseMerkleTree
	// manaRootTree stores the mana tree at the LastCommittedEpoch + 1.
	manaRootTree *smt.SparseMerkleTree
}

// NewEpochCommitmentFactory returns a new commitment factory.
func NewEpochCommitmentFactory(store kvstore.KVStore, tangle *tangle.Tangle) *EpochCommitmentFactory {
	hasher, _ := blake2b.New256(nil)

	epochCommitmentStorage := newEpochCommitmentStorage(WithStore(store))

	stateRootTreeNodeStore := objectstorage.NewStoreWithRealm(epochCommitmentStorage.baseStore, database.PrefixNotarization, PrefixStateTreeNodes)
	stateRootTreeValueStore := objectstorage.NewStoreWithRealm(epochCommitmentStorage.baseStore, database.PrefixNotarization, PrefixStateTreeValues)

	manaRootTreeStore := specializeStore(epochCommitmentStorage.baseStore, PrefixManaTree)
	manaRootTreeNodeStore := specializeStore(manaRootTreeStore, PrefixManaTreeNodes)
	manaRootTreeValueStore := specializeStore(manaRootTreeStore, PrefixManaTreeValues)

	return &EpochCommitmentFactory{
		commitmentTrees: make(map[epoch.Index]*CommitmentTrees),
		storage:         epochCommitmentStorage,
		hasher:          hasher,
		tangle:          tangle,
		stateRootTree:   smt.NewSparseMerkleTree(stateRootTreeNodeStore, stateRootTreeValueStore, hasher),
		manaRootTree:    smt.NewSparseMerkleTree(manaRootTreeNodeStore, manaRootTreeValueStore, hasher),
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
func (f *EpochCommitmentFactory) ECR(ei epoch.Index) (ecr *epoch.ECR, err error) {
	epochRoots, err := f.newEpochRoots(ei)
	if err != nil {
		return nil, errors.Wrap(err, "ECR could not be created")
	}
	branch1 := types.NewIdentifier(append(epochRoots.tangleRoot.Bytes(), epochRoots.stateMutationRoot.Bytes()...))
	branch2 := types.NewIdentifier(append(epochRoots.stateRoot.Bytes(), epochRoots.manaRoot.Bytes()...))
	root := make([]byte, 0)
	root = append(root, branch1.Bytes()...)
	root = append(root, branch2.Bytes()...)
	return &epoch.ECR{Identifier: types.NewIdentifier(root)}, nil
}

// InsertStateLeaf inserts the outputID to the state sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateLeaf(outputID utxo.OutputID) error {
	_, err := f.stateRootTree.Update(outputID.Bytes(), outputID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state tree")
	}
	return nil
}

// UpdateManaLeaf updates the mana balance in the mana sparse merkle tree.
func (f *EpochCommitmentFactory) UpdateManaLeaf(outputID utxo.OutputID, increaseBalance bool) error {
	pledgeID, balances, updatedBalances, err := f.getPledgeIDAndBalances(outputID)
	if err != nil {
		return err
	}
	var leafHasBalance bool
	balances.ForEach(func(color devnetvm.Color, balance uint64) bool {
		if increaseBalance {
			updatedBalances.Map()[color] += balance
		} else {
			updatedBalances.Map()[color] -= balance
			if updatedBalances.Map()[color] > 0 {
				leafHasBalance = true
			}
		}
		return true
	})
	// remove leaf if mana is zero
	if !leafHasBalance {
		err = f.removeManaRoot(pledgeID)
		return err
	}
	_, err = f.stateRootTree.Update(pledgeID.Bytes(), updatedBalances.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state tree")
	}
	return nil
}

func (f *EpochCommitmentFactory) getPledgeIDAndBalances(outputID utxo.OutputID) (identity.ID, *devnetvm.ColoredBalances, *devnetvm.ColoredBalances, error) {
	var pledgeID identity.ID
	f.tangle.Ledger.Storage.CachedOutputMetadata(outputID).Consume(func(metadata *ledger.OutputMetadata) {
		pledgeID = metadata.ConsensusManaPledgeID()
	})
	var balances *devnetvm.ColoredBalances
	f.tangle.Ledger.Storage.CachedOutput(outputID).Consume(func(output utxo.Output) {
		balances = output.(devnetvm.Output).Balances()
	})
	balanceBytes, err := f.stateRootTree.Get(pledgeID.Bytes())
	if err != nil {
		return identity.ID{}, nil, nil, errors.Wrap(err, "could not get leaf from mana tree")
	}
	updatedBalances, _, err := devnetvm.ColoredBalancesFromBytes(balanceBytes)
	if err != nil {
		return identity.ID{}, nil, nil, errors.Wrap(err, "could not decode balances")
	}
	return pledgeID, balances, updatedBalances, nil
}

// InsertStateMutationLeaf inserts the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateMutationLeaf(ei epoch.Index, txID utxo.TransactionID) error {
	f.commitmentsMutex.Lock()
	defer f.commitmentsMutex.Unlock()

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
func (f *EpochCommitmentFactory) RemoveStateMutationLeaf(ei epoch.Index, txID utxo.TransactionID) error {
	f.commitmentsMutex.Lock()
	defer f.commitmentsMutex.Unlock()

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
func (f *EpochCommitmentFactory) InsertTangleLeaf(ei epoch.Index, msgID tangle.MessageID) error {
	f.commitmentsMutex.Lock()
	defer f.commitmentsMutex.Unlock()

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
func (f *EpochCommitmentFactory) RemoveTangleLeaf(ei epoch.Index, msgID tangle.MessageID) error {
	f.commitmentsMutex.Lock()
	defer f.commitmentsMutex.Unlock()

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

// RemoveStateLeaf removes the output ID from the ledger sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveStateLeaf(outputID utxo.OutputID) error {
	exists, _ := f.stateRootTree.Has(outputID.Bytes())
	if exists {
		_, err := f.stateRootTree.Delete(outputID.Bytes())
		if err != nil {
			return errors.Wrap(err, "could not delete leaf from the state tree")
		}
	}
	return nil
}

func (f *EpochCommitmentFactory) removeManaRoot(id identity.ID) error {
	exists, _ := f.stateRootTree.Has(id.Bytes())
	if exists {
		_, err := f.stateRootTree.Delete(id.Bytes())
		if err != nil {
			return errors.Wrap(err, "could not delete leaf from the state tree")
		}
	}
	return nil
}

	stateRoot, manaRoot, commitmentTreesErr := f.newStateRoots(ei)
	copy(commitmentRoots.manaRoot.Bytes(), manaRoot)
// ProofStateRoot returns the merkle proof for the outputID against the state root.
func (f *EpochCommitmentFactory) ProofStateRoot(ei epoch.Index, outID utxo.OutputID) (*CommitmentProof, error) {
	key := outID.Bytes()
	root := f.commitmentTrees[ei].tangleTree.Root()
	proof, err := f.stateRootTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the state root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

// ProofStateMutationRoot returns the merkle proof for the transactionID against the state mutation root.
func (f *EpochCommitmentFactory) ProofStateMutationRoot(ei epoch.Index, txID utxo.TransactionID) (*CommitmentProof, error) {
	key := txID.Bytes()
	root := f.commitmentTrees[ei].stateMutationTree.Root()
	proof, err := f.commitmentTrees[ei].stateMutationTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the state mutation root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

// ProofTangleRoot returns the merkle proof for the blockID against the tangle root.
func (f *EpochCommitmentFactory) ProofTangleRoot(ei epoch.Index, blockID tangle.MessageID) (*CommitmentProof, error) {
	key := blockID.Bytes()
	root := f.commitmentTrees[ei].tangleTree.Root()
	proof, err := f.commitmentTrees[ei].tangleTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the tangle root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

// VerifyTangleRoot verify the provided merkle proof against the tangle root.
func (f *EpochCommitmentFactory) VerifyTangleRoot(proof CommitmentProof, blockID tangle.MessageID) bool {
	key := blockID.Bytes()
	return f.verifyRoot(proof, key, key)
}

// VerifyStateMutationRoot verify the provided merkle proof against the state mutation root.
func (f *EpochCommitmentFactory) VerifyStateMutationRoot(proof CommitmentProof, transactionID utxo.TransactionID) bool {
	key := transactionID.Bytes()
	return f.verifyRoot(proof, key, key)
}

// ecRecord retrieves the epoch commitment.
func (f *EpochCommitmentFactory) ecRecord(ei epoch.Index) (ecRecord *epoch.ECRecord, err error) {
	fmt.Println(">> ecRecord", ei)
	ecRecord = epoch.NewECRecord(ei)
	if f.storage.CachedECRecord(ei).Consume(func(record *epoch.ECRecord) {
		ecRecord.SetECR(record.ECR())
		ecRecord.SetPrevEC(record.PrevEC())
	}) {
		return ecRecord, nil
	}

	// We never committed this epoch before, create and roll to a new epoch.
	ecr, err := f.ECR(ei)
	if err != nil {
		return nil, err
	}
	prevECRecord, err := f.ecRecord(ei - 1)
	if err != nil {
		return nil, err
	}
	prevEC := EC(prevECRecord)

	// Store and return.
	f.storage.CachedECRecord(ei, epoch.NewECRecord).Consume(func(e *epoch.ECRecord) {
		e.SetECR(ecr)
		e.SetPrevEC(prevEC)
		ecRecord = e
	})

	return
}

func (f *EpochCommitmentFactory) storeDiffUTXOs(ei epoch.Index, spent utxo.OutputIDs, created devnetvm.Outputs) {
	epochDiffStorage := f.storage.getEpochDiffStorage(ei)

	for it := spent.Iterator(); it.HasNext(); {
		spentOutput := it.Next()
		epochDiffStorage.created.Delete(spentOutput.Bytes())
		// TODO: maybe we can avoid having the tangle as a dependency if we only stored the spent IDs, do we need the entire output?
		f.tangle.Ledger.Storage.CachedOutput(spentOutput).Consume(func(o utxo.Output) {
			outputVM := o.(devnetvm.Output)
			epochDiffStorage.spent.Store(outputVM)
		})
	}

	for _, createdOutput := range created {
		f.tangle.Ledger.Storage.CachedOutputMetadata(createdOutput.ID()).Consume(func(outputMetadata *ledger.OutputMetadata) {
			createdOutputWithMetadata := ledger.NewOutputWithMetadata(createdOutput.ID(), createdOutput, outputMetadata)
			epochDiffStorage.created.Store(createdOutputWithMetadata)
		})
	}
}

func (f *EpochCommitmentFactory) loadDiffUTXOs(ei epoch.Index) (spent utxo.OutputIDs, created devnetvm.Outputs) {
	epochDiffStorage := f.storage.getEpochDiffStorage(ei)

	spent = utxo.NewOutputIDs()
	epochDiffStorage.spent.ForEach(func(_ []byte, cachedOutput *objectstorage.CachedObject[utxo.Output]) bool {
		cachedOutput.Consume(func(output utxo.Output) {
			spent.Add(output.ID())
		})
		return true
	})

	created = make(devnetvm.Outputs, 0)
	epochDiffStorage.created.ForEach(func(_ []byte, cachedOutputWithMetadata *objectstorage.CachedObject[*ledger.OutputWithMetadata]) bool {
		cachedOutputWithMetadata.Consume(func(outputWithMetadata *ledger.OutputWithMetadata) {
			created = append(created, outputWithMetadata.Output().(devnetvm.Output))
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

	commitmentTrees := &CommitmentTrees{
		EI:                ei,
		tangleTree:        smt.NewSparseMerkleTree(messageIDStore, messageValueStore, f.hasher),
		stateMutationTree: smt.NewSparseMerkleTree(stateMutationIDStore, stateMutationValueStore, f.hasher),
	}

	return commitmentTrees
}

// newEpochRoots creates a new commitment with the given ei, by advancing the corresponding data structures.
func (f *EpochCommitmentFactory) newEpochRoots(ei epoch.Index) (commitmentRoots *CommitmentRoots, commitmentTreesErr error) {
	fmt.Println("\t>> newEpochRoots", ei)
	// TODO: what if a node restarts and we have incomplete trees?
	commitmentTrees, commitmentTreesErr := f.getCommitmentTrees(ei)
	if commitmentTreesErr != nil {
		return nil, errors.Wrapf(commitmentTreesErr, "cannot get commitment tree for epoch %d", ei)
	}

	// We advance the StateRootTree to the next epoch.
	// This call will fail if we are trying to commit an epoch other than the next of the last committed epoch.
	stateRoot, commitmentTreesErr := f.newStateRoot(ei)
	if commitmentTreesErr != nil {
		return nil, commitmentTreesErr
	}

	// We advance the LedgerState to the next epoch.
	if err := f.storage.commitLedgerState(ei); err != nil {
		return nil, errors.Wrapf(err, "could not commit ledger state for epoch %d", ei)
	}

	commitmentRoots = &CommitmentRoots{
		EI: ei,
	}

	copy(commitmentRoots.stateRoot.Bytes(), commitmentTrees.tangleTree.Root())
	copy(commitmentRoots.stateMutationRoot.Bytes(), commitmentTrees.stateMutationTree.Root())
	copy(commitmentRoots.stateRoot.Bytes(), stateRoot)

	return commitmentRoots, nil
}

func (f *EpochCommitmentFactory) getCommitmentTrees(ei epoch.Index) (commitmentTrees *CommitmentTrees, err error) {
	commitmentTrees, ok := f.commitmentTrees[ei]
	if !ok {
		commitmentTrees = f.newCommitmentTrees(ei)
		f.commitmentTrees[ei] = commitmentTrees
	}
	return
}

func (f *EpochCommitmentFactory) verifyRoot(proof CommitmentProof, key []byte, value []byte) bool {
	return smt.VerifyProof(proof.proof, proof.root, key, value, f.hasher)
}

func (f *EpochCommitmentFactory) newStateRoots(ei epoch.Index) (stateRoot []byte, manaRoot []byte, err error) {
	fmt.Println("\t\t>> newStateRoot", ei)
	// By the time we want the state root for a specific epoch, the diff should be complete and unalterable.
	spent, created := f.loadDiffUTXOs(ei)
		return nil, nil, errors.Errorf("could not load diff for epoch %d", ei)

	// Insert  created UTXOs into the state tree.
	for _, o := range created {
		err = f.InsertStateLeaf(o.ID())
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not insert the state leaf")
		}
		err = f.UpdateManaLeaf(o.ID(), true)
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not insert the mana leaf")
		}
	}

	// Remove spent UTXOs from the state tree.
	for it := spent.Iterator(); it.HasNext(); {
		outID := it.Next()
		err = f.RemoveStateLeaf(outID)
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not remove state leaf")
		}
		err = f.UpdateManaLeaf(outID, false)
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not remove mana leaf")
		}
	}

	return f.StateRoot(), f.ManaRoot(), nil
}

func EC(ecRecord *epoch.ECRecord) *epoch.EC {
	concatenated := append(ecRecord.PrevEC().Bytes(), ecRecord.ECR().Bytes()...)
	concatenated = append(concatenated, ecRecord.EI().Bytes()...)
	return &epoch.EC{Identifier: types.NewIdentifier(concatenated)}
}

type CommitmentProof struct {
	EI    epoch.Index
	proof smt.SparseMerkleProof
	root  []byte
}
