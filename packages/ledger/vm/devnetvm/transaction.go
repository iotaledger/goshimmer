package devnetvm

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/serializer"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// region TransactionType //////////////////////////////////////////////////////////////////////////////////////////////

// TransactionType represents the payload Type of Transaction.
var TransactionType payload.Type

func init() {
	TransactionType = payload.NewType(1337, "TransactionType")

	err := serix.DefaultAPI.RegisterTypeSettings(Transaction{}, serix.TypeSettings{}.WithObjectType(uint32(new(Transaction).Type())))
	if err != nil {
		panic(fmt.Errorf("error registering Transaction type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterInterfaceObjects((*payload.Payload)(nil), new(Transaction))
	if err != nil {
		panic(fmt.Errorf("error registering Transaction as Payload interface: %w", err))
	}

	err = serix.DefaultAPI.RegisterValidators(TransactionEssenceVersion(byte(0)), validateTransactionEssenceVersionBytes, validateTransactionEssenceVersion)
	if err != nil {
		panic(fmt.Errorf("error registering TransactionEssenceVersion validators: %w", err))
	}

	InputsArrayRules := &serix.ArrayRules{
		Min:            MinInputCount,
		Max:            MaxInputCount,
		ValidationMode: serializer.ArrayValidationModeNoDuplicates | serializer.ArrayValidationModeLexicalOrdering,
	}
	err = serix.DefaultAPI.RegisterTypeSettings(make(Inputs, 0), serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint16).WithLexicalOrdering(true).WithArrayRules(InputsArrayRules))
	if err != nil {
		panic(fmt.Errorf("error registering Inputs type settings: %w", err))
	}

	OutputsArrayRules := &serix.ArrayRules{
		Min:            MinOutputCount,
		Max:            MaxOutputCount,
		ValidationMode: serializer.ArrayValidationModeNoDuplicates | serializer.ArrayValidationModeLexicalOrdering,
	}
	err = serix.DefaultAPI.RegisterTypeSettings(make(Outputs, 0), serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint16).WithLexicalOrdering(true).WithArrayRules(OutputsArrayRules))
	if err != nil {
		panic(fmt.Errorf("error registering Outputs type settings: %w", err))
	}

	err = serix.DefaultAPI.RegisterValidators(Transaction{}, validateTransactionBytes, validateTransaction)
	if err != nil {
		panic(fmt.Errorf("error registering TransactionEssence validators: %w", err))
	}
}

func validateTransactionEssenceVersion(_ context.Context, version TransactionEssenceVersion) (err error) {
	if version != 0 {
		err = errors.Errorf("failed to parse TransactionEssenceVersion (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	return nil
}

func validateTransactionEssenceVersionBytes(_ context.Context, _ []byte) (err error) {
	return
}

func validateTransaction(_ context.Context, tx Transaction) (err error) {
	maxReferencedUnlockIndex := len(tx.Essence().Inputs()) - 1
	for i, unlockBlock := range tx.UnlockBlocks() {
		switch unlockBlock.Type() {
		case SignatureUnlockBlockType:
			continue
		case ReferenceUnlockBlockType:
			if unlockBlock.(*ReferenceUnlockBlock).ReferencedIndex() > uint16(maxReferencedUnlockIndex) {
				err = errors.Errorf("unlock block %d references non-existent unlock block at index %d", i, unlockBlock.(*ReferenceUnlockBlock).ReferencedIndex())
				return
			}
		case AliasUnlockBlockType:
			if unlockBlock.(*AliasUnlockBlock).AliasInputIndex() > uint16(maxReferencedUnlockIndex) {
				err = errors.Errorf("unlock block %d references non-existent chain input at index %d", i, unlockBlock.(*AliasUnlockBlock).AliasInputIndex())
				return
			}
		}
	}

	return nil
}

func validateTransactionBytes(_ context.Context, _ []byte) (err error) {
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionIDs ///////////////////////////////////////////////////////////////////////////////////////////////

// TransactionIDs represents a collection of TransactionIDs.
type TransactionIDs map[utxo.TransactionID]types.Empty

// Clone returns a copy of the collection of TransactionIDs.
func (t TransactionIDs) Clone() (transactionIDs TransactionIDs) {
	transactionIDs = make(TransactionIDs)
	for transactionID := range t {
		transactionIDs[transactionID] = types.Void
	}

	return
}

// String returns a human-readable version of the TransactionIDs.
func (t TransactionIDs) String() (result string) {
	return "TransactionIDs(" + strings.Join(t.Base58s(), ",") + ")"
}

// Base58s returns a slice of base58 encoded versions of the contained TransactionIDs.
func (t TransactionIDs) Base58s() (transactionIDs []string) {
	transactionIDs = make([]string, 0, len(t))
	for transactionID := range t {
		transactionIDs = append(transactionIDs, transactionID.Base58())
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Transaction //////////////////////////////////////////////////////////////////////////////////////////////////

// Transaction represents a payload that executes a value transfer in the ledger state.
type Transaction struct {
	transactionInner `serix:"0"`
}
type transactionInner struct {
	id           *utxo.TransactionID
	idMutex      sync.RWMutex
	Essence      *TransactionEssence `serix:"1"`
	UnlockBlocks UnlockBlocks        `serix:"2,lengthPrefixType=uint16"`

	objectstorage.StorableObjectFlags
}

func (t *Transaction) Inputs() (inputs []utxo.Input) {
	inputs = make([]utxo.Input, 0)
	for _, input := range t.transactionInner.Essence.Inputs() {
		inputs = append(inputs, input)
	}

	return inputs
}

// NewTransaction creates a new Transaction from the given details.
func NewTransaction(essence *TransactionEssence, unlockBlocks UnlockBlocks) (transaction *Transaction) {
	if len(unlockBlocks) != len(essence.Inputs()) {
		panic(fmt.Sprintf("in NewTransaction: Amount of UnlockBlocks (%d) does not match amount of Inputs (%d)", len(unlockBlocks), len(essence.Inputs())))
	}

	transaction = &Transaction{
		transactionInner{
			Essence:      essence,
			UnlockBlocks: unlockBlocks,
		},
	}

	SetOutputID(essence, transaction.ID())

	return
}

// FromObjectStorage creates an Transaction from sequences of key and bytes.
func (t *Transaction) FromObjectStorage(key, value []byte) error {
	tx := t
	if tx == nil {
		tx = new(Transaction)
	}

	_, err := serix.DefaultAPI.Decode(context.Background(), value, tx, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Transaction: %w", err)
		return err
	}
	transactionID := new(utxo.TransactionID)
	_, err = serix.DefaultAPI.Decode(context.Background(), key, transactionID, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Transaction.id: %w", err)
		return err
	}
	tx.transactionInner.id = transactionID

	SetOutputID(tx.Essence(), tx.ID())

	return err
}

// FromBytes unmarshals a Transaction from a sequence of bytes.
func (t *Transaction) FromBytes(data []byte) (*Transaction, error) {
	tx := new(Transaction)
	if t != nil {
		tx = t
	}
	_, err := serix.DefaultAPI.Decode(context.Background(), data, tx)
	if err != nil {
		err = errors.Errorf("failed to parse Transaction: %w", err)
		return tx, err
	}
	SetOutputID(tx.Essence(), tx.ID())

	return tx, nil
}

// ID returns the identifier of the Transaction. Since calculating the TransactionID is a resource intensive operation
// we calculate this value lazy and use double-checked locking.
func (t *Transaction) ID() utxo.TransactionID {
	t.idMutex.RLock()
	if t.id != nil {
		defer t.idMutex.RUnlock()

		return *t.id
	}

	t.idMutex.RUnlock()
	t.idMutex.Lock()
	defer t.idMutex.Unlock()

	if t.id != nil {
		return *t.id
	}

	return utxo.NewTransactionID(t.Bytes())
}

func (t *Transaction) SetID(id utxo.TransactionID) {
	t.idMutex.Lock()
	defer t.idMutex.Unlock()

	t.id = &id
}

// Type returns the Type of the Payload.
func (t *Transaction) Type() payload.Type {
	return TransactionType
}

// Essence returns the TransactionEssence of the Transaction.
func (t *Transaction) Essence() *TransactionEssence {
	return t.transactionInner.Essence
}

// UnlockBlocks returns the UnlockBlocks of the Transaction.
func (t *Transaction) UnlockBlocks() UnlockBlocks {
	return t.transactionInner.UnlockBlocks
}

// Bytes returns a marshaled version of the Transaction.
func (t *Transaction) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), t)
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// String returns a human-readable version of the Transaction.
func (t *Transaction) String() string {
	return stringify.Struct("Transaction",
		stringify.StructField("id", t.ID()),
		stringify.StructField("essence", t.Essence()),
		stringify.StructField("unlockBlocks", t.UnlockBlocks()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (t *Transaction) ObjectStorageKey() []byte {
	return t.ID().Bytes()
}

// ObjectStorageValue marshals the Transaction into a sequence of bytes. The ID is not serialized here as it is only
// used as a key in the ObjectStorage.
func (t *Transaction) ObjectStorageValue() []byte {
	return t.Bytes()
}

// SetOutputID assigns TransactionID to all outputs in TransactionEssence
func SetOutputID(essence *TransactionEssence, transactionID utxo.TransactionID) {
	for i, output := range essence.Outputs() {
		// the first call of transaction.ID() will also create a transaction id
		output.SetID(utxo.NewOutputID(transactionID, uint16(i)))
		// check if an alias output is deadlocked to itself
		// for origin alias outputs, alias address is only known once the ID of the output is set. However unlikely it is,
		// it is still possible to pre-mine a transaction with an origin alias output that has its governing or state
		// address set as the later determined alias address. Hence this check here.
		if output.Type() == AliasOutputType {
			alias := output.(*AliasOutput)
			aliasAddress := alias.GetAliasAddress()
			if alias.GetStateAddress().Equals(aliasAddress) {
				panic(fmt.Sprintf("state address of alias output at index %d (id: %s) cannot be its own alias address", i, alias.ID().Base58()))
			}
			if alias.GetGoverningAddress().Equals(aliasAddress) {
				panic(fmt.Sprintf("governing address of alias output at index %d (id: %s) cannot be its own alias address", i, alias.ID().Base58()))
			}
		}
	}
}

// code contract (make sure the struct implements all required methods)
var _ payload.Payload = new(Transaction)

var _ utxo.Transaction = new(Transaction)

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(Transaction)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionEssence ///////////////////////////////////////////////////////////////////////////////////////////

// TransactionEssence contains the transfer related information of the Transaction (without the unlocking details).
type TransactionEssence struct {
	transactionEssenceInner `serix:"0"`
}
type transactionEssenceInner struct {
	Version TransactionEssenceVersion `serix:"0"`
	// timestamp is the timestamp of the transaction.
	Timestamp time.Time `serix:"1"`
	// accessPledgeID is the nodeID to which access mana of the transaction is pledged.
	AccessPledgeID identity.ID `serix:"2"`
	// consensusPledgeID is the nodeID to which consensus mana of the transaction is pledged.
	ConsensusPledgeID identity.ID     `serix:"3"`
	Inputs            Inputs          `serix:"4,lengthPrefixType=uint16"`
	Outputs           Outputs         `serix:"5,lengthPrefixType=uint16"`
	Payload           payload.Payload `serix:"6,optional"`
}

// NewTransactionEssence creates a new TransactionEssence from the given details.
func NewTransactionEssence(
	version TransactionEssenceVersion,
	timestamp time.Time,
	accessPledgeID identity.ID,
	consensusPledgeID identity.ID,
	inputs Inputs,
	outputs Outputs,
) *TransactionEssence {
	return &TransactionEssence{
		transactionEssenceInner{
			Version:           version,
			Timestamp:         timestamp,
			AccessPledgeID:    accessPledgeID,
			ConsensusPledgeID: consensusPledgeID,
			Inputs:            inputs,
			Outputs:           outputs,
		},
	}
}

// TransactionEssenceFromBytes unmarshals a TransactionEssence from a sequence of bytes.
func TransactionEssenceFromBytes(data []byte) (transactionEssence *TransactionEssence, consumedBytes int, err error) {
	transactionEssence = new(TransactionEssence)
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), data, transactionEssence, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse TransactionEssence: %w", err)
		return
	}
	return
}

// SetPayload set the optional Payload of the TransactionEssence.
func (t *TransactionEssence) SetPayload(p payload.Payload) {
	t.transactionEssenceInner.Payload = p
}

// Version returns the Version of the TransactionEssence.
func (t *TransactionEssence) Version() TransactionEssenceVersion {
	return t.transactionEssenceInner.Version
}

// Timestamp returns the timestamp of the TransactionEssence.
func (t *TransactionEssence) Timestamp() time.Time {
	return t.transactionEssenceInner.Timestamp
}

// AccessPledgeID returns the access mana pledge nodeID of the TransactionEssence.
func (t *TransactionEssence) AccessPledgeID() identity.ID {
	return t.transactionEssenceInner.AccessPledgeID
}

// ConsensusPledgeID returns the consensus mana pledge nodeID of the TransactionEssence.
func (t *TransactionEssence) ConsensusPledgeID() identity.ID {
	return t.transactionEssenceInner.ConsensusPledgeID
}

// Inputs returns the Inputs of the TransactionEssence.
func (t *TransactionEssence) Inputs() Inputs {
	return t.transactionEssenceInner.Inputs
}

// Outputs returns the Outputs of the TransactionEssence.
func (t *TransactionEssence) Outputs() Outputs {
	return t.transactionEssenceInner.Outputs
}

// Payload returns the optional Payload of the TransactionEssence.
func (t *TransactionEssence) Payload() payload.Payload {
	return t.transactionEssenceInner.Payload
}

// Bytes returns a marshaled version of the TransactionEssence.
func (t *TransactionEssence) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), t)
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// String returns a human-readable version of the TransactionEssence.
func (t *TransactionEssence) String() string {
	return stringify.Struct("TransactionEssence",
		stringify.StructField("Version", t.transactionEssenceInner.Version),
		stringify.StructField("Timestamp", t.transactionEssenceInner.Timestamp),
		stringify.StructField("AccessPledgeID", t.transactionEssenceInner.AccessPledgeID),
		stringify.StructField("ConsensusPledgeID", t.transactionEssenceInner.ConsensusPledgeID),
		stringify.StructField("Inputs", t.transactionEssenceInner.Inputs),
		stringify.StructField("Outputs", t.transactionEssenceInner.Outputs),
		stringify.StructField("Payload", t.transactionEssenceInner.Payload),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionEssenceVersion ////////////////////////////////////////////////////////////////////////////////////

// TransactionEssenceVersion represents a version number for the TransactionEssence which can be used to ensure backward
// compatibility if the structure ever needs to get changed.
type TransactionEssenceVersion uint8

// Bytes returns a marshaled version of the TransactionEssenceVersion.
func (t TransactionEssenceVersion) Bytes() []byte {
	return []byte{byte(t)}
}

// Compare offers a comparator for TransactionEssenceVersions which returns -1 if the other TransactionEssenceVersion is
// bigger, 1 if it is smaller and 0 if they are the same.
func (t TransactionEssenceVersion) Compare(other TransactionEssenceVersion) int {
	switch {
	case t < other:
		return -1
	case t > other:
		return 1
	default:
		return 0
	}
}

// String returns a human-readable version of the TransactionEssenceVersion.
func (t TransactionEssenceVersion) String() string {
	return "TransactionEssenceVersion(" + strconv.Itoa(int(t)) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
