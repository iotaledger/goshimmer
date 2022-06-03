package notarization

import (
	"hash"
	"sync"

	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/kvstore"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

type ECR = [32]byte
type EC = [32]byte

type Commitment struct {
	EI                epoch.EI
	tangleRoot        [32]byte
	stateMutationRoot [32]byte
	stateRoot         [32]byte
	prevECR           [32]byte
}

// CommitmentTrees is a compressed form of all the information (messages and confirmed value payloads) of an epoch.
type CommitmentTrees struct {
	EI                epoch.EI
	tangleTree        *smt.SparseMerkleTree
	stateMutationTree *smt.SparseMerkleTree
	prevECR           [32]byte
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

	ecc                map[epoch.EI]EC
	lastCommittedEpoch epoch.EI

	storage        *EpochCommitmentStorage
	FullEpochIndex epoch.EI
	DiffEpochIndex epoch.EI

	// The state tree that always lags behind and gets the diffs applied to upon epoch commitment.
	stateRootTree *smt.SparseMerkleTree

	hasher hash.Hash
}

// NewEpochCommitmentFactory returns a new commitment factory.
func NewEpochCommitmentFactory(store kvstore.KVStore, vm vm.VM) *EpochCommitmentFactory {
	hasher, _ := blake2b.New256(nil)

	epochCommitmentStorage := newEpochCommitmentStorage(WithStore(store), WithVM(vm))

	return &EpochCommitmentFactory{
		commitmentTrees: make(map[epoch.EI]*CommitmentTrees),
		storage:         epochCommitmentStorage,
		hasher:          hasher,
	}
}

// StateRoot returns the root of the state sparse merkle tree.
func (f *EpochCommitmentFactory) StateRoot() []byte {
	return f.stateRootTree.Root()
}

// ECR generates the epoch commitment root.
func (f *EpochCommitmentFactory) ECR(ei epoch.EI) (ECR, error) {
	commitment, err := f.GetCommitment(ei)
	if err != nil {
		return [32]byte{}, err
	}
	branch1 := blake2b.Sum256(append(commitment.prevECR[:], commitment.tangleRoot[:]...))
	branch2 := blake2b.Sum256(append(commitment.stateRoot[:], commitment.stateMutationRoot[:]...))
	var root []byte
	root = append(root, branch1[:]...)
	root = append(root, branch2[:]...)
	return blake2b.Sum256(root), nil
}

func (f *EpochCommitmentFactory) ECHash(ei epoch.EI) EC {
	if ec, ok := f.ecc[ei]; ok {
		return ec
	}
	ECR, err := f.ECR(ei)
	if err != nil {

	}
	prevEC := f.ECHash(ei - 1)

	ECHash := blake2b.Sum256(append(append(prevEC[:], ECR[:]...), byte(ei)))
	f.ecc[ei] = ECHash
	return ECHash
}

// InsertTangleLeaf inserts msg to the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) InsertTangleLeaf(ei epoch.EI, msgID tangle.MessageID) error {
	commitment, err := f.getOrCreateCommitment(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting tangle leaf")
	}
	_, err = commitment.tangleTree.Update(msgID.Bytes(), msgID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the tangle tree")
	}
	err = f.updatePrevECR(commitment.EI)
	if err != nil {
		return errors.Wrap(err, "could not update prevECR while inserting tangle leaf")
	}
	return nil
}

// InsertStateLeaf inserts the outputID to the state sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateLeaf(ei epoch.EI, outputID utxo.OutputID) error {
	commitment, err := f.getOrCreateCommitment(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting state leaf")
	}
	_, err = f.stateRootTree.Update(outputID.Bytes(), outputID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state tree")
	}
	err = f.updatePrevECR(commitment.EI)
	if err != nil {
		return errors.Wrap(err, "could not update prevECR while inserting state leaf")
	}
	return nil
}

// InsertStateMutationLeaf inserts the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateMutationLeaf(ei epoch.EI, txID utxo.TransactionID) error {
	commitment, err := f.getOrCreateCommitment(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting state mutation leaf")
	}
	_, err = commitment.stateMutationTree.Update(txID.Bytes(), txID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state mutation tree")
	}
	err = f.updatePrevECR(commitment.EI)
	if err != nil {
		return errors.Wrap(err, "could not update prevECR while inserting state mutation leaf")
	}
	return nil
}

// RemoveStateMutationLeaf deletes the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveStateMutationLeaf(ei epoch.EI, txID utxo.TransactionID) error {
	commitment, err := f.getOrCreateCommitment(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while deleting state mutation leaf")
	}
	_, err = commitment.stateMutationTree.Delete(txID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not delete leaf from the state mutation tree")
	}
	err = f.updatePrevECR(commitment.EI)
	if err != nil {
		return errors.Wrap(err, "could not update prevECR while deleting state mutation leaf")
	}
	return nil
}

// RemoveTangleLeaf removes the message ID from the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveTangleLeaf(ei epoch.EI, msgID tangle.MessageID) error {
	commitment, err := f.getOrCreateCommitment(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while deleting tangle leaf")
	}
	exists, _ := commitment.tangleTree.Has(msgID.Bytes())
	if exists {
		_, err2 := commitment.tangleTree.Delete(msgID.Bytes())
		if err2 != nil {
			return errors.Wrap(err, "could not delete leaf from the tangle tree")
		}
		err2 = f.updatePrevECR(commitment.EI)
		if err2 != nil {
			return errors.Wrap(err, "could not update prevECR while deleting tangle leaf")
		}
	}
	return nil
}

// RemoveStateLeaf removes the output ID from the ledger sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveStateLeaf(ei epoch.EI, outID utxo.OutputID) error {
	exists, _ := f.stateRootTree.Has(outID.Bytes())
	if exists {
		_, err := f.stateRootTree.Delete(outID.Bytes())
		if err != nil {
			return errors.Wrap(err, "could not delete leaf from the state tree")
		}
		err = f.updatePrevECR(ei)
		if err != nil {
			return errors.Wrap(err, "could not update prevECR while deleting state leaf")
		}
	}
	return nil
}

// GetCommitment returns the commitment with the given ei.
func (f *EpochCommitmentFactory) GetCommitment(ei epoch.EI) (*Commitment, error) {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	commitmentTrees := f.commitmentTrees[ei]
	stateRoot, err := f.getStateRoot(ei)
	if err != nil {
		return nil, err
	}
	commitment := &Commitment{}
	commitment.EI = ei
	copy(commitment.stateRoot[:], commitmentTrees.tangleTree.Root())
	copy(commitment.stateMutationRoot[:], commitmentTrees.stateMutationTree.Root())
	copy(commitment.stateRoot[:], stateRoot)
	commitment.prevECR = commitmentTrees.prevECR

	return commitment, nil
}

// GetEpochCommitment returns the epoch commitment with the given ei.
func (f *EpochCommitmentFactory) GetEpochCommitment(ei epoch.EI) (*tangle.EpochCommitment, error) {
	ecr, err := f.ECR(ei)
	if err != nil {
		return nil, errors.Wrapf(err, "ECR could not be created for epoch %d", ei)
	}
	return &tangle.EpochCommitment{
		EI:         uint64(ei),
		ECR:        ecr,
		PreviousEC: f.ECHash(ei - 1),
	}, nil
}

func (f *EpochCommitmentFactory) ProofStateRoot(ei epoch.EI, outID utxo.OutputID) (*CommitmentProof, error) {
	key := outID.Bytes()
	root := f.commitmentTrees[ei].tangleTree.Root()
	proof, err := f.stateRootTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the state root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

func (f *EpochCommitmentFactory) ProofStateMutationRoot(ei epoch.EI, txID utxo.TransactionID) (*CommitmentProof, error) {
	key := txID.Bytes()
	root := f.commitmentTrees[ei].stateMutationTree.Root()
	proof, err := f.commitmentTrees[ei].stateMutationTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the state mutation root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

func (f *EpochCommitmentFactory) ProofTangleRoot(ei epoch.EI, blockID tangle.MessageID) (*CommitmentProof, error) {
	key := blockID.Bytes()
	root := f.commitmentTrees[ei].tangleTree.Root()
	proof, err := f.commitmentTrees[ei].tangleTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the tangle root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

func (f *EpochCommitmentFactory) getOrCreateCommitment(ei epoch.EI) (commitmentTrees *CommitmentTrees, err error) {
	f.commitmentsMutex.RLock()
	commitmentTrees, ok := f.commitmentTrees[ei]
	f.commitmentsMutex.RUnlock()
	if !ok {
		var previousECR [32]byte

		if ei > 0 {
			previousECR, err = f.ECR(ei - 1)
			if err != nil {
				return nil, err
			}
		}
		commitmentTrees = f.newCommitmentTrees(ei, previousECR)
		f.commitmentsMutex.Lock()
		f.commitmentTrees[ei] = commitmentTrees
		f.commitmentsMutex.Unlock()
	}
	return
}

// NewCommitment returns an empty commitment for the epoch.
func (f *EpochCommitmentFactory) newCommitmentTrees(ei epoch.EI, prevECR [32]byte) *CommitmentTrees {
	// Volatile storage for small trees
	db, _ := database.NewMemDB()
	messageIDStore := db.NewStore()
	messageValueStore := db.NewStore()
	stateMutationIDStore := db.NewStore()
	stateMutationValueStore := db.NewStore()

	commitment := &CommitmentTrees{
		EI:                ei,
		tangleTree:        smt.NewSparseMerkleTree(messageIDStore, messageValueStore, f.hasher),
		stateMutationTree: smt.NewSparseMerkleTree(stateMutationIDStore, stateMutationValueStore, f.hasher),
		prevECR:           prevECR,
	}

	return commitment
}

func (f *EpochCommitmentFactory) VerifyTangleRoot(proof CommitmentProof, blockID tangle.MessageID) bool {
	key := blockID.Bytes()
	return f.verifyRoot(proof, key, key)
}

func (f *EpochCommitmentFactory) VerifyStateMutationRoot(proof CommitmentProof, transactionID utxo.TransactionID) bool {
	key := transactionID.Bytes()
	return f.verifyRoot(proof, key, key)
}

func (f *EpochCommitmentFactory) verifyRoot(proof CommitmentProof, key []byte, value []byte) bool {
	return smt.VerifyProof(proof.proof, proof.root, key, value, f.hasher)
}

func (f *EpochCommitmentFactory) updatePrevECR(prevEI epoch.EI) error {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()

	forwardCommitment, ok := f.commitmentTrees[prevEI+1]
	if !ok {
		return nil
	}
	prevECR, err := f.ECR(prevEI)
	if err != nil {
		return errors.Wrap(err, "could not update previous ECR")
	}
	forwardCommitment.prevECR = prevECR
	return nil
}

func (f *EpochCommitmentFactory) getStateRoot(ei epoch.EI) ([]byte, error) {
	if ei != f.lastCommittedEpoch+1 {
		return []byte{}, errors.Errorf("getting the state root of not next committable epoch is not supported")
	}
	return f.stateRootTree.Root(), nil
}

type CommitmentProof struct {
	EI    epoch.EI
	proof smt.SparseMerkleProof
	root  []byte
}
