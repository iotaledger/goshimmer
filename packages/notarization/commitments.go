package notarization

import (
	"hash"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm"
	"github.com/iotaledger/hive.go/kvstore"

	"github.com/celestiaorg/smt"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// Commitment is a compressed form of all the information (messages and confirmed value payloads) of an epoch.
type Commitment struct {
	EI                EI
	tangleRoot        *smt.SparseMerkleTree
	stateMutationRoot *smt.SparseMerkleTree
	stateRoot         *smt.SparseMerkleTree
	prevECR           [32]byte
}

// TangleRoot returns the root of the tangle sparse merkle tree.
func (e *Commitment) TangleRoot() []byte {
	return e.tangleRoot.Root()
}

// StateMutationRoot returns the root of the state mutation sparse merkle tree.
func (e *Commitment) StateMutationRoot() []byte {
	return e.stateMutationRoot.Root()
}

// StateRoot returns the root of the state sparse merkle tree.
func (e *Commitment) StateRoot() []byte {
	return e.stateRoot.Root()
}

// EpochCommitmentFactory manages epoch commitments.
type EpochCommitmentFactory struct {
	commitments      map[EI]*Commitment
	commitmentsMutex sync.RWMutex

	ecc map[EI][32]byte

	storage *EpochCommitmentStorage

	// The state tree that always lags behind and gets the diffs applied to upon epoch commitment.
	stateRootTree *smt.SparseMerkleTree

	hasher hash.Hash
}

// NewEpochCommitmentFactory returns a new commitment factory.
func NewEpochCommitmentFactory(store kvstore.KVStore, vm vm.VM) *EpochCommitmentFactory {
	hasher, _ := blake2b.New256(nil)

	epochCommitmentStorage := newEpochCommitmentStorage(WithStore(store), WithVM(vm))

	return &EpochCommitmentFactory{
		commitments: make(map[EI]*Commitment),
		storage:     epochCommitmentStorage,
		hasher:      hasher,
	}
}

// StateRoot returns the root of the state sparse merkle tree.
func (f *EpochCommitmentFactory) StateRoot() []byte {
	return f.stateRootTree.Root()
}

// ECR generates the epoch commitment root.
func (f *EpochCommitmentFactory) ECR(ei EI) [32]byte {
	commitment := f.GetCommitment(ei)
	if commitment == nil {
		return [32]byte{}
	}
	branch1 := blake2b.Sum256(append(commitment.prevECR[:], commitment.TangleRoot()...))
	branch2 := blake2b.Sum256(append(commitment.StateRoot(), commitment.StateMutationRoot()...))
	var root []byte
	root = append(root, branch1[:]...)
	root = append(root, branch2[:]...)
	return blake2b.Sum256(root)
}

func (f *EpochCommitmentFactory) ECHash(ei EI) [32]byte {
	if ec, ok := f.ecc[ei]; ok {
		return ec
	}
	ECR := f.ECR(ei)
	prevEC := f.ECHash(ei - 1)

	ECHash := blake2b.Sum256(append(append(prevEC[:], ECR[:]...), byte(ei)))
	f.ecc[ei] = ECHash
	return ECHash
}

// InsertTangleLeaf inserts msg to the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) InsertTangleLeaf(ei EI, msgID tangle.MessageID) error {
	commitment := f.getOrCreateCommitment(ei)
	_, err := commitment.tangleRoot.Update(msgID.Bytes(), msgID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the tangle tree")
	}
	f.updatePrevECR(commitment.EI)
	return nil
}

// InsertStateLeaf inserts the outputID to the state sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateLeaf(ei EI, outputID utxo.OutputID) error {
	commitment := f.getOrCreateCommitment(ei)
	_, err := f.stateRootTree.Update(outputID.Bytes(), outputID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state tree")
	}
	f.updatePrevECR(commitment.EI)
	return nil
}

// InsertStateMutationLeaf inserts the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateMutationLeaf(ei EI, txID utxo.TransactionID) error {
	commitment := f.getOrCreateCommitment(ei)
	_, err := commitment.stateMutationRoot.Update(txID.Bytes(), txID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state mutation tree")
	}
	f.updatePrevECR(commitment.EI)
	return nil
}

// RemoveStateMutationLeaf deletes the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveStateMutationLeaf(ei EI, txID utxo.TransactionID) error {
	commitment := f.getOrCreateCommitment(ei)
	_, err := commitment.stateMutationRoot.Delete(txID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not delete leaf from the state mutation tree")
	}
	f.updatePrevECR(commitment.EI)
	return nil
}

// RemoveTangleLeaf removes the message ID from the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveTangleLeaf(ei EI, msgID tangle.MessageID) error {
	commitment := f.getOrCreateCommitment(ei)
	exists, _ := commitment.tangleRoot.Has(msgID.Bytes())
	if exists {
		_, err := commitment.tangleRoot.Delete(msgID.Bytes())
		if err != nil {
			return errors.Wrap(err, "could not delete leaf from the tangle tree")
		}
		f.updatePrevECR(commitment.EI)
	}
	return nil
}

// RemoveStateLeaf removes the output ID from the ledger sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveStateLeaf(ei EI, outID utxo.OutputID) error {
	commitment := f.getOrCreateCommitment(ei)
	exists, _ := f.stateRootTree.Has(outID.Bytes())
	if exists {
		_, err := f.stateRootTree.Delete(outID.Bytes())
		if err != nil {
			return errors.Wrap(err, "could not delete leaf from the state tree")
		}
		f.updatePrevECR(commitment.EI)
	}
	return nil
}

// GetCommitment returns the commitment with the given ei.
func (f *EpochCommitmentFactory) GetCommitment(ei EI) *Commitment {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	commitment := f.commitments[ei]
	commitment.stateRoot = f.getStateRoot(ei)
	return commitment
}

// GetEpochCommitment returns the epoch commitment with the given ei.
func (f *EpochCommitmentFactory) GetEpochCommitment(ei EI) *tangle.EpochCommitment {
	return &tangle.EpochCommitment{
		EI:         uint64(ei),
		ECR:        f.ECR(ei),
		PreviousEC: f.ECHash(ei - 1),
	}
}

func (f *EpochCommitmentFactory) ProofStateRoot(ei EI, outID utxo.OutputID) (*CommitmentProof, error) {
	key := outID.Bytes()
	root := f.commitments[ei].tangleRoot.Root()
	proof, err := f.stateRootTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the state root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

func (f *EpochCommitmentFactory) ProofStateMutationRoot(ei EI, txID utxo.TransactionID) (*CommitmentProof, error) {
	key := txID.Bytes()
	root := f.commitments[ei].stateMutationRoot.Root()
	proof, err := f.commitments[ei].stateMutationRoot.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the state mutation root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

func (f *EpochCommitmentFactory) ProofTangleRoot(ei EI, blockID tangle.MessageID) (*CommitmentProof, error) {
	key := blockID.Bytes()
	root := f.commitments[ei].tangleRoot.Root()
	proof, err := f.commitments[ei].tangleRoot.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the tangle root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

func (f *EpochCommitmentFactory) getOrCreateCommitment(ei EI) *Commitment {
	f.commitmentsMutex.RLock()
	commitment, ok := f.commitments[ei]
	f.commitmentsMutex.RUnlock()
	if !ok {
		var previousECR [32]byte

		if ei > 0 {
			previousECR = f.ECR(ei - 1)
		}
		commitment = f.newCommitment(ei, previousECR)
		f.commitmentsMutex.Lock()
		f.commitments[ei] = commitment
		f.commitmentsMutex.Unlock()
	}
	return commitment
}

// NewCommitment returns an empty commitment for the epoch.
func (f *EpochCommitmentFactory) newCommitment(ei EI, prevECR [32]byte) *Commitment {
	// Volatile storage for small trees
	db, _ := database.NewMemDB()
	messageIDStore := db.NewStore()
	messageValueStore := db.NewStore()
	stateMutationIDStore := db.NewStore()
	stateMutationValueStore := db.NewStore()

	commitment := &Commitment{
		EI:                ei,
		tangleRoot:        smt.NewSparseMerkleTree(messageIDStore, messageValueStore, f.hasher),
		stateMutationRoot: smt.NewSparseMerkleTree(stateMutationIDStore, stateMutationValueStore, f.hasher),
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

func (f *EpochCommitmentFactory) updatePrevECR(prevEI EI) {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()

	forwardCommitment, ok := f.commitments[prevEI+1]
	if !ok {
		return
	}
	forwardCommitment.prevECR = f.ECR(prevEI)
}

type CommitmentProof struct {
	EI    EI
	proof smt.SparseMerkleProof
	root  []byte
}
