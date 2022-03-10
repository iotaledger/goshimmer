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
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// region TransactionType //////////////////////////////////////////////////////////////////////////////////////////////

// TransactionType represents the payload Type of a Transaction.
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

// FromObjectStorage creates a Transaction from sequences of key and bytes.
func (t *Transaction) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	transaction, err := t.FromBytes(bytes)
	if err != nil {
		err = errors.Errorf("failed to parse Transaction from bytes: %w", err)
		return transaction, err
	}

	transactionID, _, err := TransactionIDFromBytes(key)
	if err != nil {
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
	branchIDs               BranchIDs
	branchIDsMutex          sync.RWMutex
	solid                   bool
	solidMutex              sync.RWMutex
	solidificationTime      time.Time
	solidificationTimeMutex sync.RWMutex
	lazyBooked              bool
	lazyBookedMutex         sync.RWMutex
	gradeOfFinality         gof.GradeOfFinality
	gradeOfFinalityTime     time.Time
	gradeOfFinalityMutex    sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewTransactionMetadata creates a new empty TransactionMetadata object.
func NewTransactionMetadata(transactionID TransactionID) *TransactionMetadata {
	return &TransactionMetadata{
		id:        transactionID,
		branchIDs: NewBranchIDs(),
	}
}

// FromObjectStorage creates an TransactionMetadata from sequences of key and bytes.
func (t *TransactionMetadata) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	transactionMetadata, err := t.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse TransactionMetadata from bytes: %w", err)
		return transactionMetadata, err
	}

	return transactionMetadata, err
}

// FromBytes unmarshals an TransactionMetadata object from a sequence of bytes.
func (t *TransactionMetadata) FromBytes(bytes []byte) (transactionMetadata *TransactionMetadata, err error) {
	marshalUtil := marshalutil.New(bytes)
	if transactionMetadata, err = t.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionMetadata from MarshalUtil: %w", err)
		return
	}
	return
}

// FromMarshalUtil unmarshals an TransactionMetadata object using a MarshalUtil (for easier unmarshaling).
func (t *TransactionMetadata) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (transactionMetadata *TransactionMetadata, err error) {
	if transactionMetadata = t; transactionMetadata == nil {
		transactionMetadata = &TransactionMetadata{}
	}
	if transactionMetadata.id, err = TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionID: %w", err)
		return
	}
	if transactionMetadata.branchIDs, err = BranchIDsFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchID: %w", err)
		return
	}
	if transactionMetadata.solid, err = marshalUtil.ReadBool(); err != nil {
		err = errors.Errorf("failed to parse solid flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if transactionMetadata.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		err = errors.Errorf("failed to parse solidification time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if transactionMetadata.lazyBooked, err = marshalUtil.ReadBool(); err != nil {
		err = errors.Errorf("failed to parse lazy booked flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	gradeOfFinality, err := marshalUtil.ReadUint8()
	if err != nil {
		err = errors.Errorf("failed to parse grade of finality (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	transactionMetadata.gradeOfFinality = gof.GradeOfFinality(gradeOfFinality)
	if transactionMetadata.gradeOfFinalityTime, err = marshalUtil.ReadTime(); err != nil {
		err = errors.Errorf("failed to parse gradeOfFinality time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// ID returns the TransactionID of the Transaction that the TransactionMetadata belongs to.
func (t *TransactionMetadata) ID() TransactionID {
	return t.id
}

// BranchIDs returns the identifiers of the Branches that the Transaction was booked in.
func (t *TransactionMetadata) BranchIDs() BranchIDs {
	t.branchIDsMutex.RLock()
	defer t.branchIDsMutex.RUnlock()

	return t.branchIDs.Clone()
}

// SetBranchIDs sets the identifiers of the Branches that the Transaction was booked in.
func (t *TransactionMetadata) SetBranchIDs(branchIDs BranchIDs) (modified bool) {
	t.branchIDsMutex.Lock()
	defer t.branchIDsMutex.Unlock()

	if t.branchIDs.Equals(branchIDs) {
		return false
	}

	t.branchIDs = branchIDs.Clone()
	t.SetModified()
	return true
}

// AddBranchID adds an identifier of the Branch that the Transaction was booked in.
func (t *TransactionMetadata) AddBranchID(branchID BranchID) (modified bool) {
	t.branchIDsMutex.Lock()
	defer t.branchIDsMutex.Unlock()

	if t.branchIDs.Contains(branchID) {
		return false
	}

	delete(t.branchIDs, MasterBranchID)

	t.branchIDs.Add(branchID)
	t.SetModified()
	return true
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

// LazyBooked returns a boolean flag that indicates if the Transaction has been analyzed regarding the conflicting
// status of its consumed Branches.
func (t *TransactionMetadata) LazyBooked() (lazyBooked bool) {
	t.lazyBookedMutex.RLock()
	defer t.lazyBookedMutex.RUnlock()

	return t.lazyBooked
}

// SetLazyBooked updates the lazy booked flag of the Output. It returns true if the value was modified.
func (t *TransactionMetadata) SetLazyBooked(lazyBooked bool) (modified bool) {
	t.lazyBookedMutex.Lock()
	defer t.lazyBookedMutex.Unlock()

	if t.lazyBooked == lazyBooked {
		return
	}

	t.lazyBooked = lazyBooked
	t.SetModified()
	modified = true

	return
}

// GradeOfFinality returns the grade of finality.
func (t *TransactionMetadata) GradeOfFinality() gof.GradeOfFinality {
	t.gradeOfFinalityMutex.RLock()
	defer t.gradeOfFinalityMutex.RUnlock()
	return t.gradeOfFinality
}

// SetGradeOfFinality updates the grade of finality. It returns true if it was modified.
func (t *TransactionMetadata) SetGradeOfFinality(gradeOfFinality gof.GradeOfFinality) (modified bool) {
	t.gradeOfFinalityMutex.Lock()
	defer t.gradeOfFinalityMutex.Unlock()

	if t.gradeOfFinality == gradeOfFinality {
		return
	}

	t.gradeOfFinality = gradeOfFinality
	t.gradeOfFinalityTime = clock.SyncedTime()
	t.SetModified()
	modified = true
	return
}

// GradeOfFinalityTime returns the time when the Transaction's gradeOfFinality was set.
func (t *TransactionMetadata) GradeOfFinalityTime() time.Time {
	t.gradeOfFinalityMutex.RLock()
	defer t.gradeOfFinalityMutex.RUnlock()

	return t.gradeOfFinalityTime
}

// IsConflicting returns true if the Transaction is conflicting with another Transaction (has its own Branch).
func (t *TransactionMetadata) IsConflicting() bool {
	branchIDs := t.BranchIDs()
	return len(branchIDs) == 1 && branchIDs.Contains(NewBranchID(t.ID()))
}

// Bytes marshals the TransactionMetadata into a sequence of bytes.
func (t *TransactionMetadata) Bytes() []byte {
	return byteutils.ConcatBytes(t.ObjectStorageKey(), t.ObjectStorageValue())
}

// String returns a human readable version of the TransactionMetadata.
func (t *TransactionMetadata) String() string {
	return stringify.Struct("TransactionMetadata",
		stringify.StructField("id", t.ID()),
		stringify.StructField("branchID", t.BranchIDs()),
		stringify.StructField("solid", t.Solid()),
		stringify.StructField("solidificationTime", t.SolidificationTime()),
		stringify.StructField("lazyBooked", t.LazyBooked()),
		stringify.StructField("gradeOfFinality", t.GradeOfFinality()),
		stringify.StructField("gradeOfFinalityTime", t.GradeOfFinalityTime()),
	)
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
		Write(t.BranchIDs()).
		WriteBool(t.Solid()).
		WriteTime(t.SolidificationTime()).
		WriteBool(t.LazyBooked()).
		WriteUint8(uint8(t.GradeOfFinality())).
		WriteTime(t.GradeOfFinalityTime()).
		Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = &TransactionMetadata{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
