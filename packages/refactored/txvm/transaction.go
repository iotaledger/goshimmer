package txvm

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/typeutils"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// region TransactionType //////////////////////////////////////////////////////////////////////////////////////////////

// TransactionType represents the payload type of Transactions.
var TransactionType payload.Type

// init defers the initialization of the TransactionType to not have an initialization loop.
func init() {
	TransactionType = payload.NewType(1337, "TransactionType", func(data []byte) (payload.Payload, error) {
		marshalUtil := marshalutil.New(data)
		tx, err := (&Transaction{}).FromMarshalUtil(marshalUtil)
		if err != nil {
			return nil, err
		}
		if marshalUtil.ReadOffset() != len(data) {
			return nil, errors.New("not all payload bytes were consumed")
		}
		return tx, nil
	})
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
	id           *utxo.TransactionID
	idMutex      sync.RWMutex
	essence      *TransactionEssence
	unlockBlocks UnlockBlocks

	objectstorage.StorableObjectFlags
}

func (t *Transaction) Inputs() (inputs []utxo.Input) {
	inputs = make([]utxo.Input, 0)
	for _, input := range t.essence.Inputs() {
		inputs = append(inputs, input)
	}

	return inputs
}

// NewTransaction creates a new Transaction from the given details.
func NewTransaction(essence *TransactionEssence, unlockBlocks UnlockBlocks) (transaction *Transaction) {
	if len(unlockBlocks) != len(essence.Inputs()) {
		panic(fmt.Sprintf("in NewTransaction: Amount of UnlockBlocks (%d) does not match amount of Inputs (%d)", len(unlockBlocks), len(essence.inputs)))
	}

	transaction = &Transaction{
		essence:      essence,
		unlockBlocks: unlockBlocks,
	}

	for i, output := range essence.Outputs() {
		// the first call of transaction.ID() will also create a transaction id
		output.SetID(utxo.NewOutputID(transaction.ID(), uint16(i), output.Bytes()))

		// check if an alias output is deadlocked to itself
		// for origin alias outputs, alias address is only known once the ID of the output is set. However unlikely it is,
		// it is still possible to pre-mine a transaction with an origin alias output that has its governing or state
		// address set as the latter determined alias address. Hence, this check here.
		if output.Type() == AliasOutputType {
			alias := output.(*AliasOutput)
			aliasAddress := alias.GetAliasAddress()
			if alias.GetStateAddress().Equals(aliasAddress) {
				panic(fmt.Sprintf("state address of alias output at index %d (id: %s) cannot be its own alias address", i, alias.ID()))
			}
			if alias.GetGoverningAddress().Equals(aliasAddress) {
				panic(fmt.Sprintf("governing address of alias output at index %d (id: %s) cannot be its own alias address", i, alias.ID()))
			}
		}
	}

	return
}

// FromObjectStorage creates a Transaction from sequences of key and bytes.
func (t *Transaction) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	transaction, err := t.FromBytes(bytes)
	if err != nil {
		err = errors.Errorf("failed to parse Transaction from bytes: %w", err)
		return transaction, err
	}

	var transactionID utxo.TransactionID
	if _, err = transactionID.FromBytes(key); err != nil {
		err = errors.Errorf("failed to parse TransactionID from bytes: %w", err)
		return transaction, err
	}
	transaction.id = &transactionID
	return transaction, err
}

// FromBytes unmarshals a Transaction from a sequence of bytes.
func (t *Transaction) FromBytes(bytes []byte) (transaction *Transaction, err error) {
	marshalUtil := marshalutil.New(bytes)
	if transaction, err = t.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Transaction from MarshalUtil: %w", err)
		return
	}
	return
}

// FromMarshalUtil unmarshals a Transaction using a MarshalUtil (for easier unmarshalling).
func (t *Transaction) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (transaction *Transaction, err error) {
	if transaction = t; transaction == nil {
		transaction = new(Transaction)
	}

	readStartOffset := marshalUtil.ReadOffset()

	payloadSize, err := marshalUtil.ReadUint32()
	if err != nil {
		err = errors.Errorf("failed to parse payload size from MarshalUtil (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	// a payloadSize of 0 indicates the payload is omitted and the payload is nil
	if payloadSize == 0 {
		return
	}
	payloadType, err := payload.TypeFromMarshalUtil(marshalUtil)
	if err != nil {
		err = errors.Errorf("failed to parse payload Type from MarshalUtil: %w", err)
		return
	}
	if payloadType != TransactionType {
		err = errors.Errorf("payload type '%s' does not match expected '%s': %w", payloadType, TransactionType, cerrors.ErrParseBytesFailed)
		return
	}

	if transaction.essence, err = TransactionEssenceFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionEssence from MarshalUtil: %w", err)
		return
	}
	if transaction.unlockBlocks, err = UnlockBlocksFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse UnlockBlocks from MarshalUtil: %w", err)
		return
	}

	parsedBytes := marshalUtil.ReadOffset() - readStartOffset
	if parsedBytes != int(payloadSize)+marshalutil.Uint32Size {
		err = errors.Errorf("parsed bytes (%d) did not match expected size (%d): %w", parsedBytes, payloadSize, cerrors.ErrParseBytesFailed)
		return
	}

	if len(transaction.unlockBlocks) != len(transaction.essence.Inputs()) {
		err = errors.Errorf("In TransactionFromMarshalUtil: amount of UnlockBlocks (%d) does not match amount of Inputs (%d): %w", len(transaction.unlockBlocks), len(transaction.essence.inputs), cerrors.ErrParseBytesFailed)
		return
	}

	maxReferencedUnlockIndex := len(transaction.essence.Inputs()) - 1
	for i, unlockBlock := range transaction.unlockBlocks {
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

	for i, output := range transaction.essence.Outputs() {
		output.SetID(utxo.NewOutputID(transaction.ID(), uint16(i), output.Bytes()))
		// check if an alias output is deadlocked to itself
		// for origin alias outputs, alias address is only known once the ID of the output is set
		if output.Type() == AliasOutputType {
			alias := output.(*AliasOutput)
			aliasAddress := alias.GetAliasAddress()
			if alias.GetStateAddress().Equals(aliasAddress) {
				err = errors.Errorf("state address of alias output at index %d (id: %s) cannot be its own alias address", i, alias.ID())
				return
			}
			if alias.GetGoverningAddress().Equals(aliasAddress) {
				err = errors.Errorf("governing address of alias output at index %d (id: %s) cannot be its own alias address", i, alias.ID())
				return
			}
		}
	}

	return
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

	txID := utxo.TransactionID(blake2b.Sum256(t.Bytes()))
	t.id = &txID

	return txID
}

// Type returns the Type of the Payload.
func (t *Transaction) Type() payload.Type {
	return TransactionType
}

// Essence returns the TransactionEssence of the Transaction.
func (t *Transaction) Essence() *TransactionEssence {
	return t.essence
}

// UnlockBlocks returns the UnlockBlocks of the Transaction.
func (t *Transaction) UnlockBlocks() UnlockBlocks {
	return t.unlockBlocks
}

// Bytes returns a marshaled version of the Transaction.
func (t *Transaction) Bytes() []byte {
	if t == nil {
		// if the payload is nil (i.e. when used as an optional payload) we encode that by setting the length to 0.
		return marshalutil.New(marshalutil.Uint32Size).WriteUint32(0).Bytes()
	}

	payloadBytes := byteutils.ConcatBytes(TransactionType.Bytes(), t.essence.Bytes(), t.unlockBlocks.Bytes())
	payloadBytesLength := len(payloadBytes)

	return marshalutil.New(marshalutil.Uint32Size + payloadBytesLength).
		WriteUint32(uint32(payloadBytesLength)).
		WriteBytes(payloadBytes).
		Bytes()
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

// code contract (make sure the struct implements all required methods)
var _ payload.Payload = &Transaction{}

var _ utxo.Transaction = new(Transaction)

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = &Transaction{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionEssence ///////////////////////////////////////////////////////////////////////////////////////////

// TransactionEssence contains the transfer related information of the Transaction (without the unlocking details).
type TransactionEssence struct {
	version TransactionEssenceVersion
	// timestamp is the timestamp of the transaction.
	timestamp time.Time
	// accessPledgeID is the nodeID to which access mana of the transaction is pledged.
	accessPledgeID identity.ID
	// consensusPledgeID is the nodeID to which consensus mana of the transaction is pledged.
	consensusPledgeID identity.ID
	inputs            Inputs
	outputs           Outputs
	payload           payload.Payload
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
		version:           version,
		timestamp:         timestamp,
		accessPledgeID:    accessPledgeID,
		consensusPledgeID: consensusPledgeID,
		inputs:            inputs,
		outputs:           outputs,
	}
}

// TransactionEssenceFromBytes unmarshals a TransactionEssence from a sequence of bytes.
func TransactionEssenceFromBytes(bytes []byte) (transactionEssence *TransactionEssence, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if transactionEssence, err = TransactionEssenceFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionEssence from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TransactionEssenceFromMarshalUtil unmarshals a TransactionEssence using a MarshalUtil (for easier unmarshalling).
func TransactionEssenceFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (transactionEssence *TransactionEssence, err error) {
	transactionEssence = &TransactionEssence{}
	if transactionEssence.version, err = TransactionEssenceVersionFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionEssenceVersion from MarshalUtil: %w", err)
		return
	}
	// unmarshal timestamp
	if transactionEssence.timestamp, err = marshalUtil.ReadTime(); err != nil {
		err = errors.Errorf("failed to parse Transaction timestamp from MarshalUtil: %w", err)
		return
	}
	// unmarshal accessPledgeID
	var accessPledgeIDBytes []byte
	if accessPledgeIDBytes, err = marshalUtil.ReadBytes(len(identity.ID{})); err != nil {
		err = errors.Errorf("failed to parse accessPledgeID from MarshalUtil: %w", err)
		return
	}
	copy(transactionEssence.accessPledgeID[:], accessPledgeIDBytes)

	// unmarshal consensusPledgeIDBytes
	var consensusPledgeIDBytes []byte
	if consensusPledgeIDBytes, err = marshalUtil.ReadBytes(len(identity.ID{})); err != nil {
		err = errors.Errorf("failed to parse consensusPledgeID from MarshalUtil: %w", err)
		return
	}
	copy(transactionEssence.consensusPledgeID[:], consensusPledgeIDBytes)

	if transactionEssence.inputs, err = InputsFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Inputs from MarshalUtil: %w", err)
		return
	}
	if transactionEssence.outputs, err = OutputsFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Outputs from MarshalUtil: %w", err)
		return
	}
	if transactionEssence.payload, err = payload.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Payload from MarshalUtil: %w", err)
		return
	}

	return
}

// SetPayload set the optional Payload of the TransactionEssence.
func (t *TransactionEssence) SetPayload(p payload.Payload) {
	t.payload = p
}

// Version returns the Version of the TransactionEssence.
func (t *TransactionEssence) Version() TransactionEssenceVersion {
	return t.version
}

// Timestamp returns the timestamp of the TransactionEssence.
func (t *TransactionEssence) Timestamp() time.Time {
	return t.timestamp
}

// AccessPledgeID returns the access mana pledge nodeID of the TransactionEssence.
func (t *TransactionEssence) AccessPledgeID() identity.ID {
	return t.accessPledgeID
}

// ConsensusPledgeID returns the consensus mana pledge nodeID of the TransactionEssence.
func (t *TransactionEssence) ConsensusPledgeID() identity.ID {
	return t.consensusPledgeID
}

// Inputs returns the Inputs of the TransactionEssence.
func (t *TransactionEssence) Inputs() Inputs {
	return t.inputs
}

// Outputs returns the Outputs of the TransactionEssence.
func (t *TransactionEssence) Outputs() Outputs {
	return t.outputs
}

// Payload returns the optional Payload of the TransactionEssence.
func (t *TransactionEssence) Payload() payload.Payload {
	return t.payload
}

// Bytes returns a marshaled version of the TransactionEssence.
func (t *TransactionEssence) Bytes() []byte {
	marshalUtil := marshalutil.New().
		Write(t.version).
		WriteTime(t.timestamp).
		Write(t.accessPledgeID).
		Write(t.consensusPledgeID).
		Write(t.inputs).
		Write(t.outputs)

	if !typeutils.IsInterfaceNil(t.payload) {
		marshalUtil.Write(t.payload)
	} else {
		marshalUtil.WriteUint32(0)
	}

	return marshalUtil.Bytes()
}

// String returns a human-readable version of the TransactionEssence.
func (t *TransactionEssence) String() string {
	return stringify.Struct("TransactionEssence",
		stringify.StructField("version", t.version),
		stringify.StructField("timestamp", t.timestamp),
		stringify.StructField("accessPledgeID", t.accessPledgeID),
		stringify.StructField("consensusPledgeID", t.consensusPledgeID),
		stringify.StructField("inputs", t.inputs),
		stringify.StructField("outputs", t.outputs),
		stringify.StructField("payload", t.payload),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionEssenceVersion ////////////////////////////////////////////////////////////////////////////////////

// TransactionEssenceVersion represents a version number for the TransactionEssence which can be used to ensure backward
// compatibility if the structure ever needs to get changed.
type TransactionEssenceVersion uint8

// TransactionEssenceVersionFromMarshalUtil unmarshals a TransactionEssenceVersion using a MarshalUtil (for easier
// unmarshalling).
func TransactionEssenceVersionFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (version TransactionEssenceVersion, err error) {
	readByte, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Errorf("failed to parse TransactionEssenceVersion (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if readByte != 0 {
		err = errors.Errorf("invalid TransactionVersion (%d): %w", readByte, cerrors.ErrParseBytesFailed)
		return
	}
	version = TransactionEssenceVersion(readByte)

	return
}

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
