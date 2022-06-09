package notarization

import (
	"hash"
	"sync"

	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"

	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// CommitmentRoots contains roots of trees of an epoch.
type CommitmentRoots struct {
	EI                epoch.EI
	tangleRoot        epoch.MerkleRoot
	stateMutationRoot epoch.MerkleRoot
	stateRoot         epoch.MerkleRoot
}

// CommitmentTrees is a compressed form of all the information (messages and confirmed value payloads) of an epoch.
type CommitmentTrees struct {
	EI                epoch.EI
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
	commitmentTrees  map[epoch.EI]*CommitmentTrees
	commitmentsMutex sync.RWMutex

	ecc map[epoch.EI]*epoch.EC

	storage *EpochCommitmentStorage
	tangle  *tangle.Tangle
	hasher  hash.Hash

	// stateRootTree stores the state tree at the LastCommittedEpoch.
	stateRootTree *smt.SparseMerkleTree

	Events *FactoryEvents
}

// NewEpochCommitmentFactory returns a new commitment factory.
func NewEpochCommitmentFactory(store kvstore.KVStore, vm vm.VM, tangle *tangle.Tangle) *EpochCommitmentFactory {
	hasher, _ := blake2b.New256(nil)

	epochCommitmentStorage := newEpochCommitmentStorage(WithStore(store), WithVM(vm))

	stateRootTreeStore := specializeStore(epochCommitmentStorage.baseStore, PrefixTree)
	stateRootTreeNodeStore := specializeStore(stateRootTreeStore, PrefixTreeNodes)
	stateRootTreeValueStore := specializeStore(stateRootTreeStore, PrefixTreeValues)

	return &EpochCommitmentFactory{
		commitmentTrees: make(map[epoch.EI]*CommitmentTrees),
		storage:         epochCommitmentStorage,
		hasher:          hasher,
		tangle:          tangle,
		stateRootTree:   smt.NewSparseMerkleTree(stateRootTreeNodeStore, stateRootTreeValueStore, hasher),
		Events: &FactoryEvents{
			NewCommitmentTreesCreated: event.New[*CommitmentTreesCreatedEvent](),
		},
	}
}

// StateRoot returns the root of the state sparse merkle tree at the latest committed epoch.
func (f *EpochCommitmentFactory) StateRoot() []byte {
	return f.stateRootTree.Root()
}

func (f *EpochCommitmentFactory) SetFullEpochIndex(ei epoch.EI) error {
	if err := f.storage.baseStore.Set([]byte("fullEpochIndex"), ei.Bytes()); err != nil {
		return errors.Wrap(err, "failed to set fullEpochIndex in database")
	}
	return nil
}

func (f *EpochCommitmentFactory) FullEpochIndex() (ei epoch.EI, err error) {
	var value []byte
	if value, err = f.storage.baseStore.Get([]byte("fullEpochIndex")); err != nil {
		return ei, errors.Wrap(err, "failed to get fullEpochIndex from database")
	}

	if ei, _, err = epoch.EIFromBytes(value); err != nil {
		return ei, errors.Wrap(err, "failed to deserialize EI from bytes")
	}

	return
}

func (f *EpochCommitmentFactory) SetDiffEpochIndex(ei epoch.EI) error {
	if err := f.storage.baseStore.Set([]byte("diffEpochIndex"), ei.Bytes()); err != nil {
		return errors.Wrap(err, "failed to set diffEpochIndex in database")
	}
	return nil
}

func (f *EpochCommitmentFactory) DiffEpochIndex() (ei epoch.EI, err error) {
	var value []byte
	if value, err = f.storage.baseStore.Get([]byte("diffEpochIndex")); err != nil {
		return ei, errors.Wrap(err, "failed to get diffEpochIndex from database")
	}

	if ei, _, err = epoch.EIFromBytes(value); err != nil {
		return ei, errors.Wrap(err, "failed to deserialize EI from bytes")
	}

	return
}

func (f *EpochCommitmentFactory) SetLastCommittedEpochIndex(ei epoch.EI) error {
	if err := f.storage.baseStore.Set([]byte("lastCommittedEpochIndex"), ei.Bytes()); err != nil {
		return errors.Wrap(err, "failed to set lastCommittedEpochIndex in database")
	}
	return nil
}

func (f *EpochCommitmentFactory) LastCommittedEpochIndex() (ei epoch.EI, err error) {
	var value []byte
	if value, err = f.storage.baseStore.Get([]byte("lastCommittedEpochIndex")); err != nil {
		return ei, errors.Wrap(err, "failed to get lastCommittedEpochIndex from database")
	}

	if ei, _, err = epoch.EIFromBytes(value); err != nil {
		return ei, errors.Wrap(err, "failed to deserialize EI from bytes")
	}

	return
}

// NewCommitment returns an empty commitment for the epoch.
func (f *EpochCommitmentFactory) newCommitmentTrees(ei epoch.EI) *CommitmentTrees {
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
func (f *EpochCommitmentFactory) ECR(ei epoch.EI) (ecr *epoch.ECR, err error) {
	if f.storage.CachedECRecord(ei).Consume(func(ecRecord *ECRecord) {
		ecr = ecRecord.ECR()
	}) {
		return
	}

	commitment, err := f.newCommitment(ei)
	if err != nil {
		return nil, errors.Wrap(err, "ECR could not be created")
	}
	branch1 := types.NewIdentifier(append(commitment.tangleRoot.Bytes(), commitment.stateMutationRoot.Bytes()...))
	// TODO: append mana root here...
	branch2 := types.NewIdentifier(append(commitment.stateRoot.Bytes()))
	root := make([]byte, 0)
	root = append(root, branch1.Bytes()...)
	root = append(root, branch2.Bytes()...)
	return &epoch.ECR{Identifier: types.NewIdentifier(root)}, nil
}

// EC retrieves the epoch commitment.
func (f *EpochCommitmentFactory) EC(ei epoch.EI) (ec *epoch.EC, err error) {
	if ec, ok := f.ecc[ei]; ok {
		return ec, nil
	}
	ecr, err := f.ECR(ei)
	if err != nil {
		return nil, err
	}
	prevEC, err := f.EC(ei - 1)
	if err != nil {
		return nil, err
	}

	f.storage.CachedECRecord(ei, NewECRecord).Consume(func(e *ECRecord) {
		e.SetECR(ecr)
		e.SetPrevEC(prevEC)
	})

	concatenated := append(prevEC.Bytes(), ecr.Bytes()...)
	concatenated = append(concatenated, byte(ei))
	EC := &epoch.EC{Identifier: types.NewIdentifier(concatenated)}

	f.ecc[ei] = EC

	return EC, nil
}

// InsertTangleLeaf inserts msg to the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) InsertTangleLeaf(ei epoch.EI, msgID tangle.MessageID) error {
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

// InsertStateLeaf inserts the outputID to the state sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateLeaf(outputID utxo.OutputID) error {
	_, err := f.stateRootTree.Update(outputID.Bytes(), outputID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state tree")
	}
	return nil
}

// InsertStateMutationLeaf inserts the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateMutationLeaf(ei epoch.EI, txID utxo.TransactionID) error {
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
func (f *EpochCommitmentFactory) RemoveStateMutationLeaf(ei epoch.EI, txID utxo.TransactionID) error {
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

// RemoveTangleLeaf removes the message ID from the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveTangleLeaf(ei epoch.EI, msgID tangle.MessageID) error {
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

// newCommitment creates a new commitment with the given ei, by advancing the corresponding data structures.
func (f *EpochCommitmentFactory) newCommitment(ei epoch.EI) (*CommitmentRoots, error) {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()

	// CommitmentTrees must exist at this point.
	commitmentTrees, exists := f.commitmentTrees[ei]
	if !exists {
		return nil, errors.Errorf("commitment with epoch %d does not exist", ei)
	}

	// We advance the StateRootTree to the next epoch.
	// This call will fail if we are trying to commit an epoch other than the next of the last committed epoch.
	stateRoot, err := f.newStateRoot(ei)
	if err != nil {
		return nil, err
	}
	// We advance the LedgerState to the next epoch.
	if err := f.commitLedgerState(ei); err != nil {
		return nil, errors.Wrap(err, "could not commit ledger state")
	}

	commitment := &CommitmentRoots{}
	commitment.EI = ei

	// convert []byte to [32]byte type
	copy(commitment.stateRoot.Bytes(), commitmentTrees.tangleTree.Root())
	copy(commitment.stateMutationRoot.Bytes(), commitmentTrees.stateMutationTree.Root())
	copy(commitment.stateRoot.Bytes(), stateRoot)

	return commitment, nil
}

// commitLedgerState commits the corresponding diff to the ledger state and drops it.
func (f *EpochCommitmentFactory) commitLedgerState(ei epoch.EI) (err error) {
	if !f.storage.CachedDiff(ei).Consume(func(diff *epoch.EpochDiff) {
		_ = diff.Spent().ForEach(func(spent utxo.Output) error {
			f.storage.ledgerstateStorage.Delete(spent.ID().Bytes())
			return nil
		})

		_ = diff.Created().ForEach(func(created utxo.Output) error {
			f.storage.ledgerstateStorage.Store(created).Release()
			return nil
		})
	}) {
		return errors.Errorf("could not commit ledger state for epoch %d, unavailable diff", ei)
	}

	f.storage.epochDiffStorage.Delete(ei.Bytes())

	return nil
}

// getEpochCommitment returns the epoch commitment with the given ei.
// The requested epoch must be committable.
func (f *EpochCommitmentFactory) getEpochCommitment(ei epoch.EI) (*epoch.EpochCommitment, error) {
	ecr, err := f.ECR(ei)
	if err != nil {
		return nil, errors.Wrapf(err, "epoch commitment root could not be created for epoch %d", ei)
	}
	prevEC, err := f.EC(ei - 1)
	if err != nil {
		return nil, errors.Wrapf(err, "epoch commitment could not be created for epoch %d", ei-1)
	}
	return &epoch.EpochCommitment{
		EI:         ei,
		ECR:        ecr,
		PreviousEC: prevEC,
	}, nil
}

func (f *EpochCommitmentFactory) getCommitmentTrees(ei epoch.EI) (commitmentTrees *CommitmentTrees, err error) {
	f.commitmentsMutex.RLock()
	commitmentTrees, ok := f.commitmentTrees[ei]
	f.commitmentsMutex.RUnlock()
	if !ok {
		commitmentTrees = f.newCommitmentTrees(ei)
		f.commitmentsMutex.Lock()
		f.commitmentTrees[ei] = commitmentTrees
		f.commitmentsMutex.Unlock()
		f.Events.NewCommitmentTreesCreated.Trigger(&CommitmentTreesCreatedEvent{EI: ei})
	}
	return
}

// ProofStateRoot returns the merkle proof for the outputID against the state root.
func (f *EpochCommitmentFactory) ProofStateRoot(ei epoch.EI, outID utxo.OutputID) (*CommitmentProof, error) {
	key := outID.Bytes()
	root := f.commitmentTrees[ei].tangleTree.Root()
	proof, err := f.stateRootTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the state root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

// ProofStateMutationRoot returns the merkle proof for the transactionID against the state mutation root.
func (f *EpochCommitmentFactory) ProofStateMutationRoot(ei epoch.EI, txID utxo.TransactionID) (*CommitmentProof, error) {
	key := txID.Bytes()
	root := f.commitmentTrees[ei].stateMutationTree.Root()
	proof, err := f.commitmentTrees[ei].stateMutationTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the state mutation root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

// ProofTangleRoot returns the merkle proof for the blockID against the tangle root.
func (f *EpochCommitmentFactory) ProofTangleRoot(ei epoch.EI, blockID tangle.MessageID) (*CommitmentProof, error) {
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

func (f *EpochCommitmentFactory) newStateRoot(ei epoch.EI) (stateRoot []byte, err error) {
	isNextCommittableEpoch, isNextCommittableEpochErr := f.isNextCommittableEpoch(ei)
	if isNextCommittableEpochErr != nil {
		return nil, errors.Wrap(isNextCommittableEpochErr, "could not determine if next epoch is committable")
	}
	if !isNextCommittableEpoch {
		return nil, errors.Errorf("getting the state root of not next committable epoch is not supported")
	}

	// By the time we want the state root for a specific epoch, the diff should be complete.
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

func (f *EpochCommitmentFactory) isNextCommittableEpoch(ei epoch.EI) (isNextCommittableEpoch bool, err error) {
	lastCommittedEpochIndex, lastCommittedEpochIndexErr := f.LastCommittedEpochIndex()
	if lastCommittedEpochIndexErr != nil {
		return false, errors.Errorf("failed to retrieve LastCommittedEpochIndex")
	}

	return ei == lastCommittedEpochIndex+1, nil
}

func (f *EpochCommitmentFactory) storeDiffUTXOs(ei epoch.EI, spent utxo.OutputIDs, created devnetvm.Outputs) {
	f.storage.CachedDiff(ei, epoch.NewEpochDiff).Consume(func(epochDiff *epoch.EpochDiff) {
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

func (f *EpochCommitmentFactory) loadDiffUTXOs(ei epoch.EI) (spent utxo.OutputIDs, created devnetvm.Outputs) {
	created = make(devnetvm.Outputs, 0)
	// TODO: this Load should be cached
	f.storage.CachedDiff(ei).Consume(func(epochDiff *epoch.EpochDiff) {
		spent = epochDiff.Spent().IDs()
		_ = epochDiff.Created().ForEach(func(output utxo.Output) error {
			created = append(created, output.(devnetvm.Output))
			return nil
		})
	})
	return
}

type CommitmentProof struct {
	EI    epoch.EI
	proof smt.SparseMerkleProof
	root  []byte
}

// FactoryEvents is a container that acts as a dictionary for the existing events of a commitment factory.
type FactoryEvents struct {
	// EpochCommitted is an event that gets triggered whenever a new commitment trees are created and start to be updated.
	NewCommitmentTreesCreated *event.Event[*CommitmentTreesCreatedEvent]
}

// CommitmentTreesCreatedEvent is a container that acts as a dictionary for the EpochCommitted event related parameters.
type CommitmentTreesCreatedEvent struct {
	// EI is the index of committable epoch.
	EI epoch.EI
}
