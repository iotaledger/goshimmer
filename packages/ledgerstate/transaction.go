package ledgerstate

import (
	"crypto/rand"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/xerrors"
)

// region TransactionType //////////////////////////////////////////////////////////////////////////////////////////////

// TransactionType represents the payload Type of a Transaction.
var TransactionType payload.Type

// init defers the initialization of the TransactionType to not have an initialization loop.
func init() {
	TransactionType = payload.NewType(1, "TransactionType", func(data []byte) (payload.Payload, error) {
		return TransactionFromMarshalUtil(marshalutil.New(data))
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
		err = xerrors.Errorf("failed to parse TransactionID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TransactionIDFromBase58 creates a TransactionID from a base58 encoded string.
func TransactionIDFromBase58(base58String string) (transactionID TransactionID, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded TransactionID (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if transactionID, _, err = TransactionIDFromBytes(bytes); err != nil {
		err = xerrors.Errorf("failed to parse TransactionID from bytes: %w", err)
		return
	}

	return
}

// TransactionIDFromMarshalUtil unmarshals a TransactionID using a MarshalUtil (for easier unmarshaling).
func TransactionIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (transactionID TransactionID, err error) {
	transactionIDBytes, err := marshalUtil.ReadBytes(TransactionIDLength)
	if err != nil {
		err = xerrors.Errorf("failed to parse TransactionID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(transactionID[:], transactionIDBytes)

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

// region Transaction //////////////////////////////////////////////////////////////////////////////////////////////////

// Transaction represents a payload that executes a value transfer in the ledger state.
type Transaction struct {
	id           *TransactionID
	idMutex      sync.RWMutex
	essence      *TransactionEssence
	unlockBlocks UnlockBlocks

	objectstorage.StorableObjectFlags
}

// NewTransaction creates a new Transaction from the given details.
func NewTransaction(essence *TransactionEssence, unlockBlocks UnlockBlocks) *Transaction {
	if len(unlockBlocks) != len(essence.Inputs()) {
		panic(fmt.Sprintf("amount of UnlockBlocks (%d) does not match amount of Inputs (%d)", len(unlockBlocks), len(essence.inputs)))
	}

	return &Transaction{
		essence:      essence,
		unlockBlocks: unlockBlocks,
	}
}

// TransactionFromBytes unmarshals a Transaction from a sequence of bytes.
func TransactionFromBytes(bytes []byte) (transaction *Transaction, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if transaction, err = TransactionFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Transaction from MarshalUtil: %w", err)
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
		err = xerrors.Errorf("failed to parse payload size from MarshalUtil (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	// a payloadSize of 0 indicates the payload is omitted and the payload is nil
	if payloadSize == 0 {
		return
	}
	payloadType, err := payload.TypeFromMarshalUtil(marshalUtil)
	if err != nil {
		err = xerrors.Errorf("failed to parse payload Type from MarshalUtil: %w", err)
		return
	}
	if payloadType != TransactionType {
		err = xerrors.Errorf("payload type '%s' does not match expected '%s': %w", payloadType, TransactionType, cerrors.ErrParseBytesFailed)
		return
	}

	transaction = &Transaction{}
	if transaction.essence, err = TransactionEssenceFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse TransactionEssence from MarshalUtil: %w", err)
		return
	}
	if transaction.unlockBlocks, err = UnlockBlocksFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse UnlockBlocks from MarshalUtil: %w", err)
		return
	}

	parsedBytes := marshalUtil.ReadOffset() - readStartOffset
	if parsedBytes != int(payloadSize)+marshalutil.Uint32Size {
		err = xerrors.Errorf("parsed bytes (%d) did not match expected size (%d): %w", parsedBytes, payloadSize, cerrors.ErrParseBytesFailed)
		return
	}

	if len(transaction.unlockBlocks) != len(transaction.essence.Inputs()) {
		err = xerrors.Errorf("amount of UnlockBlocks (%d) does not match amount of Inputs (%d): %w", len(transaction.unlockBlocks), len(transaction.essence.inputs), cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// TransactionFromObjectStorage restores a Transaction that was stored in the ObjectStorage.
func TransactionFromObjectStorage(key []byte, data []byte) (transaction objectstorage.StorableObject, err error) {
	if transaction, _, err = TransactionFromBytes(data); err != nil {
		err = xerrors.Errorf("failed to parse Transaction from bytes: %w", err)
		return
	}

	transactionID, _, err := TransactionIDFromBytes(key)
	if err != nil {
		err = xerrors.Errorf("failed to parse TransactionID from bytes: %w", err)
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

// Bytes returns a marshaled version of the Transaction.
func (t *Transaction) Bytes() []byte {
	if t == nil {
		// if the payload is nil (i.e. when used as an optional payload) we encode that by setting the length to 0.
		return marshalutil.New(marshalutil.Uint16Size).WriteUint16(0).Bytes()
	}

	payloadBytes := byteutils.ConcatBytes(TransactionType.Bytes(), t.essence.Bytes(), t.unlockBlocks.Bytes())
	payloadBytesLength := len(payloadBytes)

	return marshalutil.New(marshalutil.Uint16Size + payloadBytesLength).
		WriteUint16(uint16(payloadBytesLength)).
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
func (c *CachedTransaction) Consume(consumer func(cachedTransaction *Transaction), forceRelease ...bool) (consumed bool) {
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
	inputs  Inputs
	outputs Outputs
	payload payload.Payload
}

// NewTransactionEssence creates a new TransactionEssence from the given details.
func NewTransactionEssence(version TransactionEssenceVersion, inputs Inputs, outputs Outputs) *TransactionEssence {
	return &TransactionEssence{
		version: version,
		inputs:  inputs,
		outputs: outputs,
	}
}

// TransactionEssenceFromBytes unmarshals a TransactionEssence from a sequence of bytes.
func TransactionEssenceFromBytes(bytes []byte) (transactionEssence *TransactionEssence, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if transactionEssence, err = TransactionEssenceFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse TransactionEssence from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TransactionEssenceFromMarshalUtil unmarshals a TransactionEssence using a MarshalUtil (for easier unmarshaling).
func TransactionEssenceFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (transactionEssence *TransactionEssence, err error) {
	transactionEssence = &TransactionEssence{}
	if transactionEssence.version, err = TransactionEssenceVersionFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse TransactionEssenceVersion from MarshalUtil: %w", err)
		return
	}
	if transactionEssence.inputs, err = InputsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Inputs from MarshalUtil: %w", err)
		return
	}
	if transactionEssence.outputs, err = OutputsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Outputs from MarshalUtil: %w", err)
		return
	}
	if transactionEssence.payload, err = payload.FromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Payload from MarshalUtil: %w", err)
		return
	}

	return
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
		err = xerrors.Errorf("failed to parse version TransactionEssenceVersion from MarshalUtil: %w", err)
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
		err = xerrors.Errorf("failed to parse TransactionEssenceVersion (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if readByte != 0 {
		err = xerrors.Errorf("invalid TransactionVersion (%d): %w", readByte, cerrors.ErrParseBytesFailed)
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
	solid                   bool
	solidMutex              sync.RWMutex
	solidificationTime      time.Time
	solidificationTimeMutex sync.RWMutex
	preferred               bool
	preferredMutex          sync.RWMutex
	liked                   bool
	likedMutex              sync.RWMutex
	finalized               bool
	finalizedMutex          sync.RWMutex
	finalizationTime        time.Time
	finalizationTimeMutex   sync.RWMutex
	confirmed               bool
	confirmedMutex          sync.RWMutex
	rejected                bool
	rejectedMutex           sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewTransactionMetadata creates a new empty TransactionMetadata object.
func NewTransactionMetadata(transactionID TransactionID) *TransactionMetadata {
	return &TransactionMetadata{
		id: transactionID,
	}
}

// TransactionMetadataFromBytes unmarshals an TransactionMetadata object from a sequence of bytes.
func TransactionMetadataFromBytes(bytes []byte) (transactionMetadata *TransactionMetadata, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if transactionMetadata, err = TransactionMetadataFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse TransactionMetadata from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TransactionMetadataFromMarshalUtil unmarshals an TransactionMetadata object using a MarshalUtil (for easier unmarshaling).
func TransactionMetadataFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (transactionMetadata *TransactionMetadata, err error) {
	transactionMetadata = &TransactionMetadata{}
	if transactionMetadata.id, err = TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse TransactionID: %w", err)
		return
	}
	if transactionMetadata.branchID, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse BranchID: %w", err)
		return
	}
	if transactionMetadata.solid, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse solid flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if transactionMetadata.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		err = xerrors.Errorf("failed to parse solidification time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if transactionMetadata.preferred, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse preferred flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if transactionMetadata.liked, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse liked flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if transactionMetadata.finalized, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse finalized flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if transactionMetadata.finalizationTime, err = marshalUtil.ReadTime(); err != nil {
		err = xerrors.Errorf("failed to parse finalization time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if transactionMetadata.confirmed, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse confirmed flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if transactionMetadata.rejected, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse rejected flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// TransactionMetadataFromObjectStorage restores TransactionMetadata that were stored in the ObjectStorage.
func TransactionMetadataFromObjectStorage(key []byte, data []byte) (transactionMetadata objectstorage.StorableObject, err error) {
	if transactionMetadata, _, err = TransactionMetadataFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse TransactionMetadata from bytes: %w", err)
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

// Solid returns true if the Transaction has been marked as solid.
func (t *TransactionMetadata) Solid() bool {
	t.solidMutex.RLock()
	defer t.solidMutex.RUnlock()

	return t.solid
}

// SetSolid updates the solid flag of the Transaction. It returns true if the solid flag was modified and updates the
// solidification time if the Transaction was marked as solid.
func (t *TransactionMetadata) SetSolid(solid bool) (modified bool) {
	t.solidMutex.Lock()
	defer t.solidMutex.Unlock()

	if t.solid == solid {
		return
	}

	if solid {
		t.solidificationTimeMutex.Lock()
		t.solidificationTime = time.Now()
		t.solidificationTimeMutex.Unlock()
	}

	t.solid = solid
	t.SetModified()
	modified = true

	return
}

// SolidificationTime returns the time when the Transaction was marked as solid.
func (t *TransactionMetadata) SolidificationTime() time.Time {
	t.solidificationTimeMutex.RLock()
	defer t.solidificationTimeMutex.RUnlock()

	return t.solidificationTime
}

// Preferred returns true if the Transaction was marked as preferred.
func (t *TransactionMetadata) Preferred() bool {
	t.preferredMutex.RLock()
	defer t.preferredMutex.RUnlock()

	return t.preferred
}

// SetPreferred updates the preferred flag. It returns true if the value has been modified.
func (t *TransactionMetadata) SetPreferred(preferred bool) (modified bool) {
	t.preferredMutex.Lock()
	defer t.preferredMutex.Unlock()

	if t.preferred == preferred {
		return
	}

	t.preferred = preferred
	t.SetModified()
	modified = true

	return
}

// Liked returns true if the Transaction was marked as liked.
func (t *TransactionMetadata) Liked() bool {
	t.likedMutex.RLock()
	defer t.likedMutex.RUnlock()

	return t.liked
}

// SetLiked modifies the liked flag. It returns true if the value has been modified.
func (t *TransactionMetadata) SetLiked(liked bool) (modified bool) {
	t.likedMutex.Lock()
	defer t.likedMutex.Unlock()

	if t.liked == liked {
		return
	}

	t.liked = liked
	t.SetModified()
	modified = true

	return
}

// Finalized returns true if the decision if the Transaction is preferred or not has been finalized by consensus already.
func (t *TransactionMetadata) Finalized() bool {
	t.finalizedMutex.RLock()
	defer t.finalizedMutex.RUnlock()

	return t.finalized
}

// SetFinalized updates the finalized flag of the Transaction. It returns true if the value has been modified and
// updates the finalization time if the Transaction was marked as finalized.
func (t *TransactionMetadata) SetFinalized(finalized bool) (modified bool) {
	t.finalizedMutex.Lock()
	defer t.finalizedMutex.Unlock()

	if t.finalized == finalized {
		return
	}

	if finalized {
		t.finalizationTimeMutex.Lock()
		t.finalizationTime = time.Now()
		t.finalizationTimeMutex.Unlock()
	}

	t.finalized = finalized
	t.SetModified()
	modified = true

	return
}

// FinalizationTime returns the time when the Transaction was marked as finalized.
func (t *TransactionMetadata) FinalizationTime() time.Time {
	t.finalizationTimeMutex.RLock()
	defer t.finalizationTimeMutex.RUnlock()

	return t.finalizationTime
}

// Confirmed returns true if the Transaction was marked as confirmed.
func (t *TransactionMetadata) Confirmed() bool {
	t.confirmedMutex.RLock()
	defer t.confirmedMutex.RUnlock()

	return t.confirmed
}

// SetConfirmed modifies the confirmed flag. It returns true if the value has been modified.
func (t *TransactionMetadata) SetConfirmed(confirmed bool) (modified bool) {
	t.confirmedMutex.Lock()
	defer t.confirmedMutex.Unlock()

	if t.confirmed == confirmed {
		return
	}

	t.confirmed = confirmed
	t.SetModified()
	modified = true

	return
}

// Rejected returns true if the Transaction was marked as rejected.
func (t *TransactionMetadata) Rejected() bool {
	t.rejectedMutex.RLock()
	defer t.rejectedMutex.RUnlock()

	return t.rejected
}

// SetRejected modifies the rejected flag. It returns true if the value has been modified.
func (t *TransactionMetadata) SetRejected(rejected bool) (modified bool) {
	t.rejectedMutex.Lock()
	defer t.rejectedMutex.Unlock()

	if t.rejected == rejected {
		return
	}

	t.rejected = rejected
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
		stringify.StructField("branchID", t.BranchID()),
		stringify.StructField("solid", t.Solid()),
		stringify.StructField("solidificationTime", t.SolidificationTime()),
		stringify.StructField("preferred", t.Preferred()),
		stringify.StructField("liked", t.Liked()),
		stringify.StructField("finalized", t.Finalized()),
		stringify.StructField("finalizationTime", t.FinalizationTime()),
		stringify.StructField("confirmed", t.Confirmed()),
		stringify.StructField("rejected", t.Rejected()),
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
		Write(t.BranchID()).
		WriteBool(t.Solid()).
		WriteTime(t.SolidificationTime()).
		WriteBool(t.Preferred()).
		WriteBool(t.Liked()).
		WriteBool(t.Finalized()).
		WriteTime(t.FinalizationTime()).
		WriteBool(t.Confirmed()).
		WriteBool(t.Rejected()).
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
func (c *CachedTransactionMetadata) Consume(consumer func(cachedTransactionMetadata *TransactionMetadata), forceRelease ...bool) (consumed bool) {
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
