package ledgerstate

import (
	"crypto/rand"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// region TransactionType //////////////////////////////////////////////////////////////////////////////////////////////

// TransactionType represents the payload Type of a Transaction.
var TransactionType payload.Type

// init defers the initialization of the TransactionType to not have an initialization loop.
func init() {
	TransactionType = payload.NewType(1337, "TransactionType", func(data []byte) (payload.Payload, error) {
		tx, consumedBytes, err := TransactionFromBytes(data)
		if err != nil {
			return nil, err
		}
		if consumedBytes != len(data) {
			return nil, errors.New("not all payload bytes were consumed")
		}
		return tx, nil
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionID ////////////////////////////////////////////////////////////////////////////////////////////////

// TransactionIDLength contains the amount of bytes that a marshaled version of the ID contains.
const TransactionIDLength = 32

// TransactionID is the type that represents the identifier of a Transaction.
type TransactionID [TransactionIDLength]byte

// GenesisTransactionID represents the identifier of the genesis Transaction.
var GenesisTransactionID TransactionID

// TransactionIDFromBytes unmarshals a TransactionID from a sequence of bytes.
func TransactionIDFromBytes(bytes []byte) (transactionID TransactionID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if transactionID, err = TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TransactionIDFromBase58 creates a TransactionID from a base58 encoded string.
func TransactionIDFromBase58(base58String string) (transactionID TransactionID, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = errors.Errorf("error while decoding base58 encoded TransactionID (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if transactionID, _, err = TransactionIDFromBytes(bytes); err != nil {
		err = errors.Errorf("failed to parse TransactionID from bytes: %w", err)
		return
	}

	return
}

// TransactionIDFromMarshalUtil unmarshals a TransactionID using a MarshalUtil (for easier unmarshaling).
func TransactionIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (transactionID TransactionID, err error) {
	transactionIDBytes, err := marshalUtil.ReadBytes(TransactionIDLength)
	if err != nil {
		err = errors.Errorf("failed to parse TransactionID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(transactionID[:], transactionIDBytes)

	return
}

// TransactionIDsFromMarshalUtil unmarshals TransactionIDs using a MarshalUtil (for easier unmarshaling).
func TransactionIDsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (transactionIDs TransactionIDs, err error) {
	transactionIDs = make(TransactionIDs)
	for !marshalUtil.DoneReading() {
		var transactionID TransactionID
		transactionID, err = TransactionIDFromMarshalUtil(marshalUtil)
		if err != nil {
			err = errors.Errorf("failed to parse TransactionIDs: %w", err)
			return nil, err
		}
		transactionIDs[transactionID] = types.Void
	}
	return
}

// TransactionIDFromRandomness returns a random TransactionID which can for example be used in unit tests.
func TransactionIDFromRandomness() (transactionID TransactionID, err error) {
	_, err = rand.Read(transactionID[:])

	return
}

// Bytes returns a marshaled version of the TransactionID.
func (i TransactionID) Bytes() []byte {
	return i[:]
}

// Base58 returns a base58 encoded version of the TransactionID.
func (i TransactionID) Base58() string {
	return base58.Encode(i[:])
}

// String creates a human readable version of the TransactionID.
func (i TransactionID) String() string {
	return "TransactionID(" + i.Base58() + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionIDs ///////////////////////////////////////////////////////////////////////////////////////////////

// TransactionIDs represents a collection of TransactionIDs.
type TransactionIDs map[TransactionID]types.Empty

// Clone returns a copy of the collection of TransactionIDs.
func (t TransactionIDs) Clone() (transactionIDs TransactionIDs) {
	transactionIDs = make(TransactionIDs)
	for transactionID := range t {
		transactionIDs[transactionID] = types.Void
	}

	return
}

// String returns a human readable version of the TransactionIDs.
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
	id           *TransactionID
	idMutex      sync.RWMutex
	essence      *TransactionEssence
	unlockBlocks UnlockBlocks
	lock         []interface{}
	lockMutex    sync.Mutex

	objectstorage.StorableObjectFlags
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
		output.SetID(NewOutputID(transaction.ID(), uint16(i)))
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

	return
}

// TransactionFromBytes unmarshals a Transaction from a sequence of bytes.
func TransactionFromBytes(bytes []byte) (transaction *Transaction, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if transaction, err = TransactionFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Transaction from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TransactionFromMarshalUtil unmarshals a Transaction using a MarshalUtil (for easier unmarshaling).
func TransactionFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (transaction *Transaction, err error) {
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

	transaction = &Transaction{}
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
		output.SetID(NewOutputID(transaction.ID(), uint16(i)))
		// check if an alias output is deadlocked to itself
		// for origin alias outputs, alias address is only known once the ID of the output is set
		if output.Type() == AliasOutputType {
			alias := output.(*AliasOutput)
			aliasAddress := alias.GetAliasAddress()
			if alias.GetStateAddress().Equals(aliasAddress) {
				err = errors.Errorf("state address of alias output at index %d (id: %s) cannot be its own alias address", i, alias.ID().Base58())
				return
			}
			if alias.GetGoverningAddress().Equals(aliasAddress) {
				err = errors.Errorf("governing address of alias output at index %d (id: %s) cannot be its own alias address", i, alias.ID().Base58())
				return
			}
		}
	}

	return
}

// TransactionFromObjectStorage restores a Transaction that was stored in the ObjectStorage.
func TransactionFromObjectStorage(key []byte, data []byte) (transaction objectstorage.StorableObject, err error) {
	if transaction, _, err = TransactionFromBytes(data); err != nil {
		err = errors.Errorf("failed to parse Transaction from bytes: %w", err)
		return
	}

	transactionID, _, err := TransactionIDFromBytes(key)
	if err != nil {
		err = errors.Errorf("failed to parse TransactionID from bytes: %w", err)
		return
	}
	transaction.(*Transaction).id = &transactionID

	return
}

// ID returns the identifier of the Transaction. Since calculating the TransactionID is a resource intensive operation
// we calculate this value lazy and use double checked locking.
func (t *Transaction) ID() TransactionID {
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

	idBytes := blake2b.Sum256(t.Bytes())
	id, _, err := TransactionIDFromBytes(idBytes[:])
	if err != nil {
		panic(err)
	}
	t.id = &id

	return id
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

// ReferencedTransactionIDs returns a set of TransactionIDs whose Outputs were used as Inputs in this Transaction.
func (t *Transaction) ReferencedTransactionIDs() (referencedTransactionIDs TransactionIDs) {
	referencedTransactionIDs = make(TransactionIDs)
	for _, input := range t.Essence().Inputs() {
		referencedTransactionIDs[input.(*UTXOInput).ReferencedOutputID().TransactionID()] = types.Void
	}

	return
}

// Locks returns a list of identifiers of the entities that this Transaction depends on and that should be locked before
// modifying any of the Transactions properties.
func (t *Transaction) Locks() (lock []interface{}) {
	t.lockMutex.Lock()
	defer t.lockMutex.Unlock()

	if t.lock == nil {
		lockBuilder := (&syncutils.MultiMutexLockBuilder{}).AddLock(t.ID())
		for referencedTransactionID := range t.ReferencedTransactionIDs() {
			lockBuilder = lockBuilder.AddLock(referencedTransactionID)
		}

		t.lock = lockBuilder.Build()
	}

	return t.lock
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

// String returns a human readable version of the Transaction.
func (t *Transaction) String() string {
	return stringify.Struct("Transaction",
		stringify.StructField("id", t.ID()),
		stringify.StructField("essence", t.Essence()),
		stringify.StructField("unlockBlocks", t.UnlockBlocks()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (t *Transaction) Update(objectstorage.StorableObject) {
	panic("updates disabled")
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

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = &Transaction{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedTransaction ////////////////////////////////////////////////////////////////////////////////////////////

// CachedTransaction is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedTransaction struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedTransaction) Retain() *CachedTransaction {
	return &CachedTransaction{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedTransaction) Unwrap() *Transaction {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Transaction)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedTransaction) Consume(consumer func(transaction *Transaction), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Transaction))
	}, forceRelease...)
}

// String returns a human readable version of the CachedTransaction.
func (c *CachedTransaction) String() string {
	return stringify.Struct("CachedTransaction",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

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

// TransactionEssenceFromMarshalUtil unmarshals a TransactionEssence using a MarshalUtil (for easier unmarshaling).
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

// String returns a human readable version of the TransactionEssence.
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

// TransactionEssenceVersionFromBytes unmarshals a TransactionEssenceVersion from a sequence of bytes.
func TransactionEssenceVersionFromBytes(bytes []byte) (version TransactionEssenceVersion, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if version, err = TransactionEssenceVersionFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse version TransactionEssenceVersion from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TransactionEssenceVersionFromMarshalUtil unmarshals a TransactionEssenceVersion using a MarshalUtil (for easier
// unmarshaling).
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

// String returns a human readable version of the TransactionEssenceVersion.
func (t TransactionEssenceVersion) String() string {
	return "TransactionEssenceVersion(" + strconv.Itoa(int(t)) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionMetadata //////////////////////////////////////////////////////////////////////////////////////////

// TransactionMetadata contains additional information about a Transaction that is derived from the local perception of
// a node.
type TransactionMetadata struct {
	id                      TransactionID
	branchID                BranchID
	branchIDMutex           sync.RWMutex
	solidityType            SolidityType
	solidityTypeMutex       sync.RWMutex
	solidificationTime      time.Time
	solidificationTimeMutex sync.RWMutex
	finalized               bool
	finalizedMutex          sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewTransactionMetadata creates a new empty TransactionMetadata object.
func NewTransactionMetadata(transactionID TransactionID) (transactionMetadata *TransactionMetadata) {
	transactionMetadata = &TransactionMetadata{
		id: transactionID,
	}

	transactionMetadata.SetModified()
	transactionMetadata.Persist()

	return transactionMetadata
}

// TransactionMetadataFromBytes unmarshals an TransactionMetadata object from a sequence of bytes.
func TransactionMetadataFromBytes(bytes []byte) (transactionMetadata *TransactionMetadata, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if transactionMetadata, err = TransactionMetadataFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionMetadata from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TransactionMetadataFromMarshalUtil unmarshals an TransactionMetadata object using a MarshalUtil (for easier unmarshaling).
func TransactionMetadataFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (transactionMetadata *TransactionMetadata, err error) {
	transactionMetadata = &TransactionMetadata{}
	if transactionMetadata.id, err = TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionID: %w", err)
		return
	}
	if transactionMetadata.solidityType, err = SolidityTypeFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SolidityType from MarshalUtil: %w", err)
		return
	}
	if transactionMetadata.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		err = errors.Errorf("failed to parse solidification time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if transactionMetadata.branchID, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchID: %w", err)
		return
	}
	if transactionMetadata.finalized, err = marshalUtil.ReadBool(); err != nil {
		err = errors.Errorf("failed to parse finalized flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// TransactionMetadataFromObjectStorage restores TransactionMetadata that were stored in the ObjectStorage.
func TransactionMetadataFromObjectStorage(key []byte, data []byte) (transactionMetadata objectstorage.StorableObject, err error) {
	if transactionMetadata, _, err = TransactionMetadataFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse TransactionMetadata from bytes: %w", err)
		return
	}

	return
}

// ID returns the TransactionID of the Transaction that the TransactionMetadata belongs to.
func (t *TransactionMetadata) ID() TransactionID {
	return t.id
}

// BranchID returns the identifier of the Branch that the Transaction was booked in.
func (t *TransactionMetadata) BranchID() BranchID {
	t.branchIDMutex.RLock()
	defer t.branchIDMutex.RUnlock()

	return t.branchID
}

// SetBranchID sets the identifier of the Branch that the Transaction was booked in.
func (t *TransactionMetadata) SetBranchID(branchID BranchID) (modified bool) {
	t.branchIDMutex.Lock()
	defer t.branchIDMutex.Unlock()

	if t.branchID == branchID {
		return
	}

	t.branchID = branchID
	t.SetModified()
	modified = true

	return
}

// SetSolidityType updates the SolidityType of the Transaction. It returns the previous value.
func (t *TransactionMetadata) SetSolidityType(solidityType SolidityType) (previousValue SolidityType) {
	t.solidityTypeMutex.Lock()
	defer t.solidityTypeMutex.Unlock()

	if previousValue = t.solidityType; previousValue >= solidityType {
		return
	}

	if (previousValue == UndefinedSolidityType || previousValue == Unsolid) && solidityType >= LazySolid {
		t.solidificationTimeMutex.Lock()
		t.solidificationTime = time.Now()
		t.solidificationTimeMutex.Unlock()
	}

	t.solidityType = solidityType
	t.SetModified()

	return
}

// SolidityType returns the SolidityType of the given Transaction.
func (t *TransactionMetadata) SolidityType() (solidityType SolidityType) {
	t.solidityTypeMutex.RLock()
	defer t.solidityTypeMutex.RUnlock()

	return t.solidityType
}

// SolidificationTime returns the time when the Transaction was marked as solid.
func (t *TransactionMetadata) SolidificationTime() time.Time {
	t.solidificationTimeMutex.RLock()
	defer t.solidificationTimeMutex.RUnlock()

	return t.solidificationTime
}

// Finalized returns a boolean flag that indicates if the Transaction has been finalized regarding its decision of being
// included in the ledger state.
func (t *TransactionMetadata) Finalized() (finalized bool) {
	t.finalizedMutex.RLock()
	defer t.finalizedMutex.RUnlock()

	return t.finalized
}

// SetFinalized updates the finalized flag of the Transaction. It returns true if the value was modified.
func (t *TransactionMetadata) SetFinalized(finalized bool) (modified bool) {
	t.finalizedMutex.Lock()
	defer t.finalizedMutex.Unlock()

	if t.finalized == finalized {
		return
	}

	t.finalized = finalized
	t.SetModified()
	modified = true

	return
}

// Bytes marshals the TransactionMetadata into a sequence of bytes.
func (t *TransactionMetadata) Bytes() []byte {
	return byteutils.ConcatBytes(t.ObjectStorageKey(), t.ObjectStorageValue())
}

// String returns a human readable version of the TransactionMetadata.
func (t *TransactionMetadata) String() string {
	return stringify.Struct("TransactionMetadata",
		stringify.StructField("id", t.ID()),
		stringify.StructField("solidityType", t.SolidityType()),
		stringify.StructField("solidificationTime", t.SolidificationTime()),
		stringify.StructField("branchID", t.BranchID()),
		stringify.StructField("finalized", t.Finalized()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (t *TransactionMetadata) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (t *TransactionMetadata) ObjectStorageKey() []byte {
	return t.id.Bytes()
}

// ObjectStorageValue marshals the TransactionMetadata into a sequence of bytes. The ID is not serialized here as it is
// only used as a key in the ObjectStorage.
func (t *TransactionMetadata) ObjectStorageValue() []byte {
	return marshalutil.New().
		Write(t.SolidityType()).
		WriteTime(t.SolidificationTime()).
		Write(t.BranchID()).
		WriteBool(t.Finalized()).
		Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = &TransactionMetadata{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedTransactionMetadata ////////////////////////////////////////////////////////////////////////////////////

// CachedTransactionMetadata is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedTransactionMetadata struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedTransactionMetadata) Retain() *CachedTransactionMetadata {
	return &CachedTransactionMetadata{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedTransactionMetadata) Unwrap() *TransactionMetadata {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*TransactionMetadata)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedTransactionMetadata) Consume(consumer func(transactionMetadata *TransactionMetadata), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*TransactionMetadata))
	}, forceRelease...)
}

// String returns a human readable version of the CachedTransactionMetadata.
func (c *CachedTransactionMetadata) String() string {
	return stringify.Struct("CachedTransactionMetadata",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
