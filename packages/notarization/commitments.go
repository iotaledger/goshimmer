package notarization

import (
	"fmt"
	"hash"
	"sync"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"

	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"

	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// CommitmentRoots contains roots of trees of an epoch.
type CommitmentRoots struct {
	EI                epoch.Index
	tangleRoot        epoch.MerkleRoot
	stateMutationRoot epoch.MerkleRoot
	stateRoot         epoch.MerkleRoot
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
}

// NewEpochCommitmentFactory returns a new commitment factory.
func NewEpochCommitmentFactory(store kvstore.KVStore, tangle *tangle.Tangle) *EpochCommitmentFactory {
	hasher, _ := blake2b.New256(nil)

	epochCommitmentStorage := newEpochCommitmentStorage(WithStore(store))

	stateRootTreeStore := specializeStore(epochCommitmentStorage.baseStore, PrefixStateTree)
	stateRootTreeNodeStore := specializeStore(stateRootTreeStore, PrefixStateTreeNodes)
	stateRootTreeValueStore := specializeStore(stateRootTreeStore, PrefixStateTreeValues)

	return &EpochCommitmentFactory{
		commitmentTrees: make(map[epoch.Index]*CommitmentTrees),
		storage:         epochCommitmentStorage,
		hasher:          hasher,
		tangle:          tangle,
		stateRootTree:   smt.NewSparseMerkleTree(stateRootTreeNodeStore, stateRootTreeValueStore, hasher),
	}
}

// StateRoot returns the root of the state sparse merkle tree at the latest committed epoch.
func (f *EpochCommitmentFactory) StateRoot() []byte {
	return f.stateRootTree.Root()
}

func (f *EpochCommitmentFactory) SetFullEpochIndex(ei epoch.Index) error {
	if err := f.storage.baseStore.Set([]byte("fullEpochIndex"), ei.Bytes()); err != nil {
		return errors.Wrap(err, "failed to set fullEpochIndex in database")
	}
	return nil
}

func (f *EpochCommitmentFactory) FullEpochIndex() (ei epoch.Index, err error) {
	var value []byte
	if value, err = f.storage.baseStore.Get([]byte("fullEpochIndex")); err != nil {
		return ei, errors.Wrap(err, "failed to get fullEpochIndex from database")
	}

	if ei, _, err = epoch.EIFromBytes(value); err != nil {
		return ei, errors.Wrap(err, "failed to deserialize EI from bytes")
	}

	return
}

func (f *EpochCommitmentFactory) SetDiffEpochIndex(ei epoch.Index) error {
	if err := f.storage.baseStore.Set([]byte("diffEpochIndex"), ei.Bytes()); err != nil {
		return errors.Wrap(err, "failed to set diffEpochIndex in database")
	}
	return nil
}

func (f *EpochCommitmentFactory) DiffEpochIndex() (ei epoch.Index, err error) {
	var value []byte
	if value, err = f.storage.baseStore.Get([]byte("diffEpochIndex")); err != nil {
		return ei, errors.Wrap(err, "failed to get diffEpochIndex from database")
	}

	if ei, _, err = epoch.EIFromBytes(value); err != nil {
		return ei, errors.Wrap(err, "failed to deserialize EI from bytes")
	}

	return
}

func (f *EpochCommitmentFactory) SetLastCommittedEpochIndex(ei epoch.Index) error {
	if err := f.storage.baseStore.Set([]byte("lastCommittedEpochIndex"), ei.Bytes()); err != nil {
		return errors.Wrap(err, "failed to set lastCommittedEpochIndex in database")
	}
	return nil
}

func (f *EpochCommitmentFactory) LastCommittedEpochIndex() (ei epoch.Index, err error) {
	var value []byte
	if value, err = f.storage.baseStore.Get([]byte("lastCommittedEpochIndex")); err != nil {
		return ei, errors.Wrap(err, "failed to get lastCommittedEpochIndex from database")
	}

	if ei, _, err = epoch.EIFromBytes(value); err != nil {
		return ei, errors.Wrap(err, "failed to deserialize EI from bytes")
	}

	return
}

func (f *EpochCommitmentFactory) SetLastConfirmedEpochIndex(ei epoch.Index) error {
	if err := f.storage.baseStore.Set([]byte("lastConfirmedEpochIndex"), ei.Bytes()); err != nil {
		return errors.Wrap(err, "failed to set lastConfirmedEpochIndex in database")
	}
	return nil
}

func (f *EpochCommitmentFactory) LastConfirmedEpochIndex() (ei epoch.Index, err error) {
	var value []byte
	if value, err = f.storage.baseStore.Get([]byte("lastConfirmedEpochIndex")); err != nil {
		return ei, errors.Wrap(err, "failed to get lastConfirmedEpochIndex from database")
	}

	if ei, _, err = epoch.EIFromBytes(value); err != nil {
		return ei, errors.Wrap(err, "failed to deserialize EI from bytes")
	}

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

// ECR retrieves the epoch commitment root.
func (f *EpochCommitmentFactory) ECR(ei epoch.Index) (ecr *epoch.ECR, err error) {
	epochRoots, err := f.newEpochRoots(ei)
	if err != nil {
		return nil, errors.Wrap(err, "ECR could not be created")
	}
	branch1 := types.NewIdentifier(append(epochRoots.tangleRoot.Bytes(), epochRoots.stateMutationRoot.Bytes()...))
	// TODO: append mana root here...
	branch2 := types.NewIdentifier(append(epochRoots.stateRoot.Bytes()))
	root := make([]byte, 0)
	root = append(root, branch1.Bytes()...)
	root = append(root, branch2.Bytes()...)
	return &epoch.ECR{Identifier: types.NewIdentifier(root)}, nil
}

// ecRecord retrieves the epoch commitment.
func (f *EpochCommitmentFactory) ecRecord(ei epoch.Index) (ecRecord *epoch.ECRecord, err error) {
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

// InsertStateLeaf inserts the outputID to the state sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateLeaf(outputID utxo.OutputID) error {
	_, err := f.stateRootTree.Update(outputID.Bytes(), outputID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state tree")
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

// newEpochRoots creates a new commitment with the given ei, by advancing the corresponding data structures.
func (f *EpochCommitmentFactory) newEpochRoots(ei epoch.Index) (commitmentRoots *CommitmentRoots, commitmentTreesErr error) {
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
	if err := f.commitLedgerState(ei); err != nil {
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

// commitLedgerState commits the corresponding diff to the ledger state and drops it.
func (f *EpochCommitmentFactory) commitLedgerState(ei epoch.Index) (err error) {
	if !f.storage.CachedDiff(ei).Consume(func(diff *ledger.EpochDiff) {
		diff.Spent().ForEach(func(spent utxo.Output) error {
			f.storage.ledgerstateStorage.Delete(spent.ID().Bytes())
			return nil
		})

		diff.Created().ForEach(func(created utxo.Output) error {
			fmt.Println(">> commitLedgerState", created)
			f.storage.ledgerstateStorage.Store(created).Release()
			return nil
		})
	}) {
		return errors.Errorf("could not commit ledger state for epoch %d, unavailable diff", ei)
	}

	f.storage.epochDiffStorage.Delete(ei.Bytes())

	return nil
}

func (f *EpochCommitmentFactory) getCommitmentTrees(ei epoch.Index) (commitmentTrees *CommitmentTrees, err error) {
	commitmentTrees, ok := f.commitmentTrees[ei]
	if !ok {
		commitmentTrees = f.newCommitmentTrees(ei)
		f.commitmentTrees[ei] = commitmentTrees
	}
	return
}

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

func (f *EpochCommitmentFactory) verifyRoot(proof CommitmentProof, key []byte, value []byte) bool {
	return smt.VerifyProof(proof.proof, proof.root, key, value, f.hasher)
}

func (f *EpochCommitmentFactory) newStateRoot(ei epoch.Index) (stateRoot []byte, err error) {
	// By the time we want the state root for a specific epoch, the diff should be complete and unalterable.
	spent, created := f.loadDiffUTXOs(ei)
	if spent == nil || created == nil {
		return nil, errors.Errorf("could not load diff for epoch %d", ei)
	}

	// Insert created UTXOs into the state tree.
	for _, o := range created {
		err := f.InsertStateLeaf(o.ID())
		if err != nil {
			return nil, errors.Wrap(err, "could not insert the state leaf")
		}
	}

	// Remove spent UTXOs from the state tree.
	for it := spent.Iterator(); it.HasNext(); {
		err := f.RemoveStateLeaf(it.Next())
		if err != nil {
			return nil, errors.Wrap(err, "could not remove state leaf")
		}
	}

	return f.StateRoot(), nil
}

func (f *EpochCommitmentFactory) storeDiffUTXOs(ei epoch.Index, spent utxo.OutputIDs, created devnetvm.Outputs) {
	f.storage.CachedDiff(ei, ledger.NewEpochDiff).Consume(func(epochDiff *ledger.EpochDiff) {
		for _, o := range created {
			epochDiff.AddCreated(o)
		}
		for it := spent.Iterator(); it.HasNext(); {
			outputID := it.Next()
			// We won't mark the output as spent if it was created in this epoch.
			if epochDiff.DeleteCreated(outputID) {
				continue
			}
			// TODO: maybe we can avoid having the tangle as a dependency if we only stored the spent IDs, do we need the entire output?
			f.tangle.Ledger.Storage.CachedOutput(outputID).Consume(func(o utxo.Output) {
				outVM := o.(devnetvm.Output)
				epochDiff.AddSpent(outVM)
			})
		}
		epochDiff.SetModified()
		epochDiff.Persist()
	})
}

func (f *EpochCommitmentFactory) loadDiffUTXOs(ei epoch.Index) (spent utxo.OutputIDs, created devnetvm.Outputs) {
	created = make(devnetvm.Outputs, 0)
	f.storage.CachedDiff(ei, ledger.NewEpochDiff).Consume(func(epochDiff *ledger.EpochDiff) {
		spent = epochDiff.Spent().IDs()
		epochDiff.Created().ForEach(func(output utxo.Output) error {
			created = append(created, output.(devnetvm.Output))
			return nil
		})
	})
	return
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
