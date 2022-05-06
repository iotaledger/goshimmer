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

// InsertECTR inserts msg in the ECTR.
func (f *EpochCommitmentFactory) InsertECTR(eci ECI, msgID tangle.MessageID) {
	commitment := f.getOrCreateCommitment(eci)
	commitment.messageRoot.Update(msgID.Bytes(), msgID.Bytes())
	f.onMessageRootChanged(commitment)
}

// InsertECSR inserts the outputID in the ECSR.
func (f *EpochCommitmentFactory) InsertECSR(eci ECI, outputID ledgerstate.OutputID) {

}

// InsertECSMR inserts the transaction ID in the ECSMR.
func (f *EpochCommitmentFactory) InsertECSMR(eci ECI, txID ledgerstate.TransactionID) {
	commitment := f.getOrCreateCommitment(eci)
	commitment.transactionRoot.Update(txID.Bytes(), txID.Bytes())
	f.onTransactionRootChanged(commitment)
}

// RemoveECTR removes the message ID from the ECTR.
func (f *EpochCommitmentFactory) RemoveECTR(eci ECI, msgID tangle.MessageID) {
	commitment := f.getOrCreateCommitment(eci)
	commitment.messageRoot.Delete(msgID.Bytes())
	f.onMessageRootChanged(commitment)
}

// GetCommitment returns the commitment with the given eci.
func (f *EpochCommitmentFactory) GetCommitment(eci ECI) *EpochCommitment {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	return f.commitments[eci]
}

func (f *EpochCommitmentFactory) getOrCreateCommitment(eci ECI) *EpochCommitment {
	f.commitmentsMutex.Lock()
	defer f.commitmentsMutex.Unlock()
	commitment, ok := f.commitments[eci]
	if !ok {
		var previousMessageRoot []byte
		var previousTransactionRoot []byte

		if previousEpochCommitment := f.GetCommitment(eci - 1); previousEpochCommitment != nil {
			previousMessageRoot = previousEpochCommitment.MessageRoot()
			previousTransactionRoot = previousEpochCommitment.TransactionRoot()
		}
		commitment = NewEpochCommitment(eci, previousMessageRoot, previousTransactionRoot)
		f.commitments[eci] = commitment
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
