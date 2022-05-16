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
	prevMessageRootKey     = []byte("previousMessageRoot")
	prevTransactionRootKey = []byte("previousTransactionRoot")
)

// EpochCommitment is a compressed form of all the information (messages and confirmed value payloads) of a certain epoch.
type EpochCommitment struct {
	ECI             ECI
	messageRoot     *smt.SparseMerkleTree
	transactionRoot *smt.SparseMerkleTree
	ledgerStateRoot *smt.SparseMerkleTree
	manaStateRoot   *smt.SparseMerkleTree
}

// MessageRoot returns the root of the message merkle tree.
func (e *EpochCommitment) MessageRoot() []byte {
	return e.messageRoot.Root()
}

// TransactionRoot returns the root of the transaction merkle tree.
func (e *EpochCommitment) TransactionRoot() []byte {
	return e.transactionRoot.Root()
}

// LedgerStateRoot returns the root of the ledgerstate merkle tree.
func (e *EpochCommitment) LedgerStateRoot() []byte {
	return e.ledgerStateRoot.Root()
}

// NewEpochCommitment returns an empty commitment for the epoch.
func NewEpochCommitment(eci ECI, prevMessageRoot, prevTransactionRoot []byte) *EpochCommitment {
	db, _ := database.NewMemDB()
	messageIDStore := db.NewStore()
	messageValueStore := db.NewStore()
	stateIDStore := db.NewStore()
	stateValueStore := db.NewStore()
	stateMutationIDStore := db.NewStore()
	stateMutationValueStore := db.NewStore()

	hasher, _ := blake2b.New256(nil)
	commitment := &EpochCommitment{
		ECI:             eci,
		messageRoot:     smt.NewSparseMerkleTree(messageIDStore, messageValueStore, hasher),
		transactionRoot: smt.NewSparseMerkleTree(stateIDStore, stateValueStore, hasher),
		ledgerStateRoot: smt.NewSparseMerkleTree(stateMutationIDStore, stateMutationValueStore, hasher),
	}

	commitment.messageRoot.Update(prevMessageRootKey, prevMessageRoot)
	commitment.transactionRoot.Update(prevTransactionRootKey, prevTransactionRoot)
	return commitment
}

// EpochCommitmentFactory manages epoch commitments.
type EpochCommitmentFactory struct {
	commitments      map[ECI]*EpochCommitment
	commitmentsMutex sync.RWMutex
}

// NewCommitmentFactory returns a new commitment factory.
func NewCommitmentFactory() *EpochCommitmentFactory {
	return &EpochCommitmentFactory{
		commitments: make(map[ECI]*EpochCommitment),
	}
}

// InsertTangleLeaf inserts msg to the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) InsertTangleLeaf(eci ECI, msgID tangle.MessageID) {
	commitment := f.getOrCreateCommitment(eci)
	commitment.messageRoot.Update(msgID.Bytes(), msgID.Bytes())
	f.onMessageRootChanged(commitment)
}

// InsertStateLeaf inserts the outputID to the ledgerstate sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateLeaf(eci ECI, outputID ledgerstate.OutputID) {
	commitment := f.getOrCreateCommitment(eci)
	commitment.ledgerStateRoot.Update(outputID.Bytes(), outputID.Bytes())
}

// InsertStateMutationLeaf inserts the transaction ID to the transaction sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateMutationLeaf(eci ECI, txID ledgerstate.TransactionID) {
	commitment := f.getOrCreateCommitment(eci)
	commitment.transactionRoot.Update(txID.Bytes(), txID.Bytes())
	f.onTransactionRootChanged(commitment)
}

// RemoveTangleLeaf removes the message ID from the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveTangleLeaf(eci ECI, msgID tangle.MessageID) {
	commitment := f.getOrCreateCommitment(eci)
	exists, _ := commitment.ledgerStateRoot.Has(msgID.Bytes())
	if exists {
		commitment.messageRoot.Delete(msgID.Bytes())
		f.onMessageRootChanged(commitment)
	}
}

// RemoveStateLeaf removes the output ID from the ledgerstate sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveStateLeaf(eci ECI, outID ledgerstate.OutputID) {
	commitment := f.getOrCreateCommitment(eci)
	exists, _ := commitment.ledgerStateRoot.Has(outID.Bytes())
	if exists {
		commitment.ledgerStateRoot.Delete(outID.Bytes())
	}
}

// GetCommitment returns the commitment with the given eci.
func (f *EpochCommitmentFactory) GetCommitment(eci ECI) *EpochCommitment {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	return f.commitments[eci]
}

func (f *EpochCommitmentFactory) getOrCreateCommitment(eci ECI) *EpochCommitment {
	f.commitmentsMutex.RLock()
	commitment, ok := f.commitments[eci]
	f.commitmentsMutex.RUnlock()
	if !ok {
		var previousMessageRoot []byte
		var previousTransactionRoot []byte

		if eci > 0 {
			if previousEpochCommitment := f.GetCommitment(eci - 1); previousEpochCommitment != nil {
				previousMessageRoot = previousEpochCommitment.MessageRoot()
				previousTransactionRoot = previousEpochCommitment.TransactionRoot()
			}
		}
		commitment = NewEpochCommitment(eci, previousMessageRoot, previousTransactionRoot)
		f.commitmentsMutex.Lock()
		f.commitments[eci] = commitment
		f.commitmentsMutex.Unlock()
	}
	return commitment
}

func (f *EpochCommitmentFactory) onMessageRootChanged(commitment *EpochCommitment) {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	forwardCommitment, ok := f.commitments[commitment.ECI+1]
	if !ok {
		return
	}
	forwardCommitment.messageRoot.Update(prevMessageRootKey, commitment.MessageRoot())
}

func (f *EpochCommitmentFactory) onTransactionRootChanged(commitment *EpochCommitment) {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	forwardCommitment, ok := f.commitments[commitment.ECI+1]
	if !ok {
		return
	}
	forwardCommitment.transactionRoot.Update(prevTransactionRootKey, commitment.TransactionRoot())
}
