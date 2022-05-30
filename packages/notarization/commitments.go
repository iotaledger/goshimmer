package notarization

import (
	"sync"

	"github.com/lazyledger/smt"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/marshalutil"
)

var (
	prevECR = []byte("previousEpochCommitmentRoot")
)

// EpochCommitment contains the ECR and prevECR of an epoch.
type EpochCommitment struct {
	EI EI
	EC []byte
}

// Commitment is a compressed form of all the information (messages and confirmed value payloads) of an epoch.
type Commitment struct {
	EI                EI
	tangleRoot        *smt.SparseMerkleTree
	stateMutationRoot *smt.SparseMerkleTree
	stateRoot         *smt.SparseMerkleTree
	prevECR           []byte
}

// NewCommitment returns an empty commitment for the epoch.
func NewCommitment(ei EI, prevECR []byte) *Commitment {
	db, _ := database.NewMemDB()
	messageIDStore := db.NewStore()
	messageValueStore := db.NewStore()
	stateIDStore := db.NewStore()
	stateValueStore := db.NewStore()
	stateMutationIDStore := db.NewStore()
	stateMutationValueStore := db.NewStore()

	hasher, _ := blake2b.New256(nil)
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
func (e *Commitment) ECR() []byte {
	branch1 := blake2b.Sum256(append(e.prevECR, e.TangleRoot()...))
	branch2 := blake2b.Sum256(append(e.StateRoot(), e.StateMutationRoot()...))
	var root []byte
	root = append(root, branch1[:]...)
	root = append(root, branch2[:]...)
	ecr := blake2b.Sum256(root)

	return ecr[:]
}

// EpochCommitmentFactory manages epoch commitments.
type EpochCommitmentFactory struct {
	commitments      map[EI]*Commitment
	commitmentsMutex sync.RWMutex
}

// NewEpochCommitmentFactory returns a new commitment factory.
func NewEpochCommitmentFactory() *EpochCommitmentFactory {
	return &EpochCommitmentFactory{
		commitments: make(map[EI]*Commitment),
	}
}

// InsertTangleLeaf inserts msg to the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) InsertTangleLeaf(ei EI, msgID tangle.MessageID) {
	commitment := f.getOrCreateCommitment(ei)
	commitment.tangleRoot.Update(msgID.Bytes(), msgID.Bytes())
	f.onTangleRootChanged(commitment)
}

// InsertStateLeaf inserts the outputID to the state sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateLeaf(ei EI, outputID ledgerstate.OutputID) {
	commitment := f.getOrCreateCommitment(ei)
	commitment.stateRoot.Update(outputID.Bytes(), outputID.Bytes())
	f.onStateRootChanged(commitment)
}

// InsertStateMutationLeaf inserts the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateMutationLeaf(ei EI, txID ledgerstate.TransactionID) {
	commitment := f.getOrCreateCommitment(ei)
	commitment.stateMutationRoot.Update(txID.Bytes(), txID.Bytes())
	f.onStateMutationRootChanged(commitment)
}

// RemoveTangleLeaf removes the message ID from the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveTangleLeaf(ei EI, msgID tangle.MessageID) {
	commitment := f.getOrCreateCommitment(ei)
	exists, _ := commitment.stateRoot.Has(msgID.Bytes())
	if exists {
		commitment.tangleRoot.Delete(msgID.Bytes())
		f.onTangleRootChanged(commitment)
	}
}

// RemoveStateLeaf removes the output ID from the ledgerstate sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveStateLeaf(ei EI, outID ledgerstate.OutputID) {
	commitment := f.getOrCreateCommitment(ei)
	exists, _ := commitment.stateRoot.Has(outID.Bytes())
	if exists {
		commitment.stateRoot.Delete(outID.Bytes())
		f.onStateRootChanged(commitment)
	}
}

// GetCommitment returns the commitment with the given ei.
func (f *EpochCommitmentFactory) GetCommitment(ei EI) *Commitment {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	return f.commitments[ei]
}

// GetEpochCommitment returns the epoch commitment with the given ei.
func (f *EpochCommitmentFactory) GetEpochCommitment(ei EI) *EpochCommitment {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	if commitment, ok := f.commitments[ei]; ok {
		marshalUtil := marshalutil.New()

		marshalUtil.WriteBytes(commitment.prevECR)
		marshalUtil.WriteBytes(commitment.ECR())
		marshalUtil.WriteInt64(int64(ei))
		ec := blake2b.Sum256(marshalUtil.Bytes())

		return &EpochCommitment{
			EI: ei,
			EC: ec[:],
		}
	}
	return nil
}

func (f *EpochCommitmentFactory) ProofStateRoot(ei EI, outID ledgerstate.OutputID) (*CommitmentProof, error) {
	key := outID.Bytes()
	proof, err := f.commitments[ei].stateRoot.Prove(key)
	if err != nil {
		return nil, errors.Newf("could not generate the state root proof: %w", err)
	}
	return &CommitmentProof{ei, proof}, nil
}

func (f *EpochCommitmentFactory) ProofStateMutationRoot(ei EI, txID ledgerstate.TransactionID) (*CommitmentProof, error) {
	key := txID.Bytes()
	proof, err := f.commitments[ei].stateRoot.Prove(key)
	if err != nil {
		return nil, errors.Newf("could not generate the state mutation root proof: %w", err)
	}
	return &CommitmentProof{ei, proof}, nil
}

func (f *EpochCommitmentFactory) ProofTangleRoot(ei EI, blockID tangle.MessageID) (*CommitmentProof, error) {
	key := blockID.Bytes()
	proof, err := f.commitments[ei].tangleRoot.Prove(key)
	if err != nil {
		return nil, errors.Newf("could not generate the tangle root proof: %w", err)
	}
	return &CommitmentProof{ei, proof}, nil
}

func (f *EpochCommitmentFactory) getOrCreateCommitment(ei EI) *Commitment {
	f.commitmentsMutex.RLock()
	commitment, ok := f.commitments[ei]
	f.commitmentsMutex.RUnlock()
	if !ok {
		var previousECR []byte

		if ei > 0 {
			if previousCommitment := f.GetCommitment(ei - 1); previousCommitment != nil {
				previousECR = previousCommitment.ECR()
			}
		}
		commitment = NewCommitment(ei, previousECR)
		f.commitmentsMutex.Lock()
		f.commitments[ei] = commitment
		f.commitmentsMutex.Unlock()
	}
	return commitment
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
