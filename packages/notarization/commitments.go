package notarization

import (
	"sync"

	"github.com/lazyledger/smt"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

var (
	prevTangleRootKey        = []byte("previousTangleRoot")
	prevStateMutationRootKey = []byte("prevStateMutationRootKey")
	prevECR                  = []byte("previousEpochCommitmentRoot")
)

// EpochCommitment contains the ECR and prevECR of an epoch.
type EpochCommitment struct {
	ECI     ECI
	ECR     []byte
	PrevECR []byte
}

// Commitment is a compressed form of all the information (messages and confirmed value payloads) of an epoch.
type Commitment struct {
	ECI               ECI
	tangleRoot        *smt.SparseMerkleTree
	stateMutationRoot *smt.SparseMerkleTree
	stateRoot         *smt.SparseMerkleTree
	prevECR           []byte
}

// NewCommitment returns an empty commitment for the epoch.
func NewCommitment(eci ECI, prevECR, prevMessageRoot, prevTransactionRoot []byte) *Commitment {
	db, _ := database.NewMemDB()
	messageIDStore := db.NewStore()
	messageValueStore := db.NewStore()
	stateIDStore := db.NewStore()
	stateValueStore := db.NewStore()
	stateMutationIDStore := db.NewStore()
	stateMutationValueStore := db.NewStore()

	hasher, _ := blake2b.New256(nil)
	commitment := &Commitment{
		ECI:               eci,
		tangleRoot:        smt.NewSparseMerkleTree(messageIDStore, messageValueStore, hasher),
		stateMutationRoot: smt.NewSparseMerkleTree(stateIDStore, stateValueStore, hasher),
		stateRoot:         smt.NewSparseMerkleTree(stateMutationIDStore, stateMutationValueStore, hasher),
		prevECR:           prevECR,
	}

	commitment.tangleRoot.Update(prevTangleRootKey, prevMessageRoot)
	commitment.stateMutationRoot.Update(prevStateMutationRootKey, prevTransactionRoot)
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
	commitments      map[ECI]*Commitment
	commitmentsMutex sync.RWMutex
}

// NewEpochCommitmentFactory returns a new commitment factory.
func NewEpochCommitmentFactory() *EpochCommitmentFactory {
	return &EpochCommitmentFactory{
		commitments: make(map[ECI]*Commitment),
	}
}

// InsertTangleLeaf inserts msg to the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) InsertTangleLeaf(eci ECI, msgID tangle.MessageID) {
	commitment := f.getOrCreateCommitment(eci)
	commitment.tangleRoot.Update(msgID.Bytes(), msgID.Bytes())
	f.onTangleRootChanged(commitment)
}

// InsertStateLeaf inserts the outputID to the state sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateLeaf(eci ECI, outputID ledgerstate.OutputID) {
	commitment := f.getOrCreateCommitment(eci)
	commitment.stateRoot.Update(outputID.Bytes(), outputID.Bytes())
	f.onStateRootChanged(commitment)
}

// InsertStateMutationLeaf inserts the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateMutationLeaf(eci ECI, txID ledgerstate.TransactionID) {
	commitment := f.getOrCreateCommitment(eci)
	commitment.stateMutationRoot.Update(txID.Bytes(), txID.Bytes())
	f.onStateMutationRootChanged(commitment)
}

// RemoveTangleLeaf removes the message ID from the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveTangleLeaf(eci ECI, msgID tangle.MessageID) {
	commitment := f.getOrCreateCommitment(eci)
	exists, _ := commitment.stateRoot.Has(msgID.Bytes())
	if exists {
		commitment.tangleRoot.Delete(msgID.Bytes())
		f.onTangleRootChanged(commitment)
	}
}

// RemoveStateLeaf removes the output ID from the ledgerstate sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveStateLeaf(eci ECI, outID ledgerstate.OutputID) {
	commitment := f.getOrCreateCommitment(eci)
	exists, _ := commitment.stateRoot.Has(outID.Bytes())
	if exists {
		commitment.stateRoot.Delete(outID.Bytes())
		f.onStateRootChanged(commitment)
	}
}

// GetCommitment returns the commitment with the given eci.
func (f *EpochCommitmentFactory) GetCommitment(eci ECI) *Commitment {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	return f.commitments[eci]
}

// GetEpochCommitment returns the epoch commitment with the given eci.
func (f *EpochCommitmentFactory) GetEpochCommitment(eci ECI) *EpochCommitment {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	if commitment, ok := f.commitments[eci]; ok {
		return &EpochCommitment{
			ECI:     eci,
			ECR:     commitment.ECR(),
			PrevECR: commitment.prevECR,
		}
	}
	return nil
}

func (f *EpochCommitmentFactory) getOrCreateCommitment(eci ECI) *Commitment {
	f.commitmentsMutex.RLock()
	commitment, ok := f.commitments[eci]
	f.commitmentsMutex.RUnlock()
	if !ok {
		var previousMessageRoot []byte
		var previousTransactionRoot, previousECR []byte

		if eci > 0 {
			if previousCommitment := f.GetCommitment(eci - 1); previousCommitment != nil {
				previousMessageRoot = previousCommitment.TangleRoot()
				previousTransactionRoot = previousCommitment.StateMutationRoot()
				previousECR = previousCommitment.ECR()
			}
		}
		commitment = NewCommitment(eci, previousECR, previousMessageRoot, previousTransactionRoot)
		f.commitmentsMutex.Lock()
		f.commitments[eci] = commitment
		f.commitmentsMutex.Unlock()
	}
	return commitment
}

func (f *EpochCommitmentFactory) onTangleRootChanged(commitment *Commitment) {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	forwardCommitment, ok := f.commitments[commitment.ECI+1]
	if !ok {
		return
	}
	forwardCommitment.tangleRoot.Update(prevTangleRootKey, commitment.TangleRoot())
	forwardCommitment.prevECR = commitment.ECR()
}

func (f *EpochCommitmentFactory) onStateMutationRootChanged(commitment *Commitment) {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	forwardCommitment, ok := f.commitments[commitment.ECI+1]
	if !ok {
		return
	}
	forwardCommitment.stateMutationRoot.Update(prevStateMutationRootKey, commitment.StateMutationRoot())
	forwardCommitment.prevECR = commitment.ECR()
}

func (f *EpochCommitmentFactory) onStateRootChanged(commitment *Commitment) {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	forwardCommitment, ok := f.commitments[commitment.ECI+1]
	if !ok {
		return
	}
	forwardCommitment.stateRoot.Update(prevStateMutationRootKey, commitment.StateMutationRoot())
	forwardCommitment.prevECR = commitment.ECR()
}
