package notarization

import (
	"github.com/cockroachdb/errors"
	"hash"
	"sync"

	"github.com/lazyledger/smt"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
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

// NewCommitment returns an empty commitment for the epoch.
func NewCommitment(ei EI, prevECR [32]byte, hasher hash.Hash) *Commitment {
	db, _ := database.NewMemDB()
	messageIDStore := db.NewStore()
	messageValueStore := db.NewStore()
	stateIDStore := db.NewStore()
	stateValueStore := db.NewStore()
	stateMutationIDStore := db.NewStore()
	stateMutationValueStore := db.NewStore()

	commitment := &Commitment{
		EI:                ei,
		tangleRoot:        smt.NewSparseMerkleTree(messageIDStore, messageValueStore, hasher),
		stateMutationRoot: smt.NewSparseMerkleTree(stateIDStore, stateValueStore, hasher),
		stateRoot:         smt.NewSparseMerkleTree(stateMutationIDStore, stateMutationValueStore, hasher),
		prevECR:           prevECR,
	}

	return commitment
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

// ECR generates the epoch commitment root.
func (e *Commitment) ECR() [32]byte {
	branch1 := blake2b.Sum256(append(e.prevECR[:], e.TangleRoot()...))
	branch2 := blake2b.Sum256(append(e.StateRoot(), e.StateMutationRoot()...))
	var root []byte
	root = append(root, branch1[:]...)
	root = append(root, branch2[:]...)
	return blake2b.Sum256(root)

}

// EpochCommitmentFactory manages epoch commitments.
type EpochCommitmentFactory struct {
	commitments      map[EI]*Commitment
	commitmentsMutex sync.RWMutex

	hasher hash.Hash
}

// NewEpochCommitmentFactory returns a new commitment factory.
func NewEpochCommitmentFactory() *EpochCommitmentFactory {
	hasher, _ := blake2b.New256(nil)

	return &EpochCommitmentFactory{
		commitments: make(map[EI]*Commitment),
		hasher:      hasher,
	}
}

// InsertTangleLeaf inserts msg to the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) InsertTangleLeaf(ei EI, msgID tangle.MessageID) error {
	commitment := f.getOrCreateCommitment(ei)
	_, err := commitment.tangleRoot.Update(msgID.Bytes(), msgID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the tangle tree")
	}
	f.onTangleRootChanged(commitment)
	return nil
}

// InsertStateLeaf inserts the outputID to the state sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateLeaf(ei EI, outputID ledgerstate.OutputID) error {
	commitment := f.getOrCreateCommitment(ei)
	_, err := commitment.stateRoot.Update(outputID.Bytes(), outputID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state tree")
	}
	f.onStateRootChanged(commitment)
	return nil
}

// InsertStateMutationLeaf inserts the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateMutationLeaf(ei EI, txID ledgerstate.TransactionID) error {
	commitment := f.getOrCreateCommitment(ei)
	_, err := commitment.stateMutationRoot.Update(txID.Bytes(), txID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state mutation tree")
	}
	f.onStateMutationRootChanged(commitment)
	return nil
}

// RemoveTangleLeaf removes the message ID from the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveTangleLeaf(ei EI, msgID tangle.MessageID) error {
	commitment := f.getOrCreateCommitment(ei)
	exists, _ := commitment.stateRoot.Has(msgID.Bytes())
	if exists {
		_, err := commitment.tangleRoot.Delete(msgID.Bytes())
		if err != nil {
			return errors.Wrap(err, "could not delete leaf from the tangle tree")
		}
		f.onTangleRootChanged(commitment)
	}
	return nil
}

// RemoveStateLeaf removes the output ID from the ledgerstate sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveStateLeaf(ei EI, outID ledgerstate.OutputID) error {
	commitment := f.getOrCreateCommitment(ei)
	exists, _ := commitment.stateRoot.Has(outID.Bytes())
	if exists {
		_, err := commitment.stateRoot.Delete(outID.Bytes())
		if err != nil {
			return errors.Wrap(err, "could not delete leaf from the state tree")
		}
		f.onStateRootChanged(commitment)
	}
	return nil
}

// GetCommitment returns the commitment with the given ei.
func (f *EpochCommitmentFactory) GetCommitment(ei EI) *Commitment {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	return f.commitments[ei]
}

// GetEpochCommitment returns the epoch commitment with the given ei.
func (f *EpochCommitmentFactory) GetEpochCommitment(ei EI) *tangle.EpochCommitment {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	if commitment, ok := f.commitments[ei]; ok {
		return &tangle.EpochCommitment{
			EI:          uint64(ei),
			ECR:         commitment.ECR(),
			PreviousECR: commitment.prevECR,
		}
	}
	return nil
}

func (f *EpochCommitmentFactory) ProofStateRoot(ei EI, outID ledgerstate.OutputID) (*CommitmentProof, error) {
	key := outID.Bytes()
	root := f.commitments[ei].tangleRoot.Root()
	proof, err := f.commitments[ei].stateRoot.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the state root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

func (f *EpochCommitmentFactory) ProofStateMutationRoot(ei EI, txID ledgerstate.TransactionID) (*CommitmentProof, error) {
	key := txID.Bytes()
	root := f.commitments[ei].tangleRoot.Root()
	proof, err := f.commitments[ei].stateRoot.ProveForRoot(key, root)
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
			if previousCommitment := f.GetCommitment(ei - 1); previousCommitment != nil {
				previousECR = previousCommitment.ECR()
			}
		}
		commitment = NewCommitment(ei, previousECR, f.hasher)
		f.commitmentsMutex.Lock()
		f.commitments[ei] = commitment
		f.commitmentsMutex.Unlock()
	}
	return commitment
}

func (f *EpochCommitmentFactory) VerifyTangleRoot(proof CommitmentProof, blockID tangle.MessageID) bool {
	key := blockID.Bytes()
	return f.verifyRoot(proof, key, key)
}

func (f *EpochCommitmentFactory) VerifyStateMutationRoot(proof CommitmentProof, transactionID ledgerstate.TransactionID) bool {
	key := transactionID.Bytes()
	return f.verifyRoot(proof, key, key)
}

func (f *EpochCommitmentFactory) verifyRoot(proof CommitmentProof, key []byte, value []byte) bool {
	return smt.VerifyProof(proof.proof, proof.root, key, value, f.hasher)
}

func (f *EpochCommitmentFactory) onTangleRootChanged(commitment *Commitment) {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	forwardCommitment, ok := f.commitments[commitment.EI+1]
	if !ok {
		return
	}
	forwardCommitment.prevECR = commitment.ECR()
}

func (f *EpochCommitmentFactory) onStateMutationRootChanged(commitment *Commitment) {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	forwardCommitment, ok := f.commitments[commitment.EI+1]
	if !ok {
		return
	}
	forwardCommitment.prevECR = commitment.ECR()
}

func (f *EpochCommitmentFactory) onStateRootChanged(commitment *Commitment) {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	forwardCommitment, ok := f.commitments[commitment.EI+1]
	if !ok {
		return
	}
	forwardCommitment.prevECR = commitment.ECR()
}

type CommitmentProof struct {
	EI    EI
	proof smt.SparseMerkleProof
	root  []byte
}
