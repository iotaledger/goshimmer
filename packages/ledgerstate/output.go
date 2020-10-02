package ledgerstate

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/xerrors"
)

// region OutputType ///////////////////////////////////////////////////////////////////////////////////////////////////

// OutputType represents the type of an Output. Outputs of different types can have different unlock rules and allow for
// some relatively basic smart contract logic.
type OutputType uint8

const (
	// SigLockedSingleOutputType represents an Output holding vanilla IOTA tokens that gets unlocked by a signature.
	SigLockedSingleOutputType OutputType = iota

	// SigLockedColoredOutputType represents an Output that holds colored coins that gets unlocked by a signature.
	SigLockedColoredOutputType
)

// String returns a human readable representation of the OutputType.
func (o OutputType) String() string {
	return [...]string{
		"SigLockedSingleOutputType",
		"SigLockedColoredOutputType",
	}[o]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

// OutputIDLength contains the amount of bytes that a marshaled version of the OutputID contains.
const OutputIDLength = TransactionIDLength + marshalutil.UINT16_SIZE

// OutputID is the data type that represents the identifier of an Output.
type OutputID [OutputIDLength]byte

// NewOutputID is the constructor for the OutputID.
func NewOutputID(transactionID TransactionID, outputIndex uint16) (outputID OutputID) {
	copy(outputID[:TransactionIDLength], transactionID.Bytes())
	binary.LittleEndian.PutUint16(outputID[TransactionIDLength:], outputIndex)

	return
}

// OutputIDFromBytes unmarshals an OutputID from a sequence of bytes.
func OutputIDFromBytes(bytes []byte) (outputID OutputID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if outputID, err = OutputIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse OutputID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// OutputIDFromBase58 creates an OutputID from a base58 encoded string.
func OutputIDFromBase58(base58String string) (outputID OutputID, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded OutputID (%v): %w", err, ErrBase58DecodeFailed)
		return
	}

	if outputID, _, err = OutputIDFromBytes(bytes); err != nil {
		err = xerrors.Errorf("failed to parse OutputID from bytes: %w", err)
		return
	}

	return
}

// OutputIDFromMarshalUtil unmarshals an OutputID using a MarshalUtil (for easier unmarshaling).
func OutputIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (outputID OutputID, err error) {
	outputIDBytes, err := marshalUtil.ReadBytes(OutputIDLength)
	if err != nil {
		err = xerrors.Errorf("failed to parse OutputID (%v): %w", err, ErrParseBytesFailed)
		return
	}
	copy(outputID[:], outputIDBytes)

	return
}

// TransactionID returns the TransactionID part of an OutputID.
func (o OutputID) TransactionID() (transactionID TransactionID) {
	copy(transactionID[:], o[:TransactionIDLength])

	return
}

// OutputIndex returns the Output index part of an OutputID.
func (o OutputID) OutputIndex() uint16 {
	return binary.LittleEndian.Uint16(o[TransactionIDLength:])
}

// Bytes marshals the OutputID into a sequence of bytes.
func (o OutputID) Bytes() []byte {
	return o[:]
}

// Base58 returns a base58 encoded version of the OutputID.
func (o OutputID) Base58() string {
	return base58.Encode(o[:])
}

// String creates a human readable version of the OutputID.
func (o OutputID) String() string {
	return stringify.Struct("OutputID",
		stringify.StructField("transactionID", o.TransactionID()),
		stringify.StructField("outputIndex", o.OutputIndex()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Output ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Output is a generic interface for the different types of Outputs (with different unlock behaviors).
type Output interface {
	// ID returns the identifier of the Output that is used to address the Output in the UTXODAG.
	ID() OutputID

	// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
	// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
	// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
	// only accessed when the Output is supposed to be persisted.
	SetID(outputID OutputID)

	// Type returns the OutputType which allows us to generically handle Outputs of different types.
	Type() OutputType

	// Balances returns the funds that are associated with the Output.
	Balances() *ColoredBalances

	// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the
	// Output.
	UnlockValid(tx *Transaction, unlockBlock UnlockBlock) (bool, error)

	// Bytes returns a marshaled version of the Output.
	Bytes() []byte

	// String returns a human readable version of the Output for debug purposes.
	String() string

	// StorableObject makes Outputs storable in the ObjectStorage.
	objectstorage.StorableObject
}

// OutputFromBytes unmarshals an Output from a sequence of bytes.
func OutputFromBytes(bytes []byte) (output Output, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if output, err = OutputFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Output from bytes: %w", err)
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// OutputFromMarshalUtil unmarshals an Output using a MarshalUtil (for easier unmarshaling).
func OutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (output Output, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse OutputType (%v): %w", err, ErrParseBytesFailed)
		return
	}
	marshalUtil.ReadSeek(-1)

	switch OutputType(outputType) {
	case SigLockedSingleOutputType:
		if output, err = SigLockedSingleOutputFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse SigLockedSingleOutput: %w", err)
			return
		}
	case SigLockedColoredOutputType:
		if output, err = SigLockedColoredOutputFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse SigLockedColoredOutput: %w", err)
			return
		}
	default:
		err = xerrors.Errorf("unsupported OutputType (%X): %w", outputType, ErrParseBytesFailed)
		return
	}

	return
}

// OutputFromObjectStorage restores an Output that was stored in the ObjectStorage.
func OutputFromObjectStorage(key []byte, data []byte) (output Output, err error) {
	if output, _, err = OutputFromBytes(data); err != nil {
		err = xerrors.Errorf("failed to parse Output from bytes: %w", err)
		return
	}

	outputID, _, err := OutputIDFromBytes(key)
	if err != nil {
		err = xerrors.Errorf("failed to parse OutputID from bytes: %w", err)
		return
	}
	output.SetID(outputID)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SigLockedSingleOutput ////////////////////////////////////////////////////////////////////////////////////////

// SigLockedSingleOutput is an Output that holds exactly one uncolored balance and that can be unlocked by providing a
// signature for an Address.
type SigLockedSingleOutput struct {
	id      OutputID
	idMutex sync.RWMutex
	balance uint64
	address Address

	objectstorage.StorableObjectFlags
}

// NewSigLockedSingleOutput is the constructor for a SigLockedSingleOutput.
func NewSigLockedSingleOutput(balance uint64, address Address) *SigLockedSingleOutput {
	return &SigLockedSingleOutput{
		balance: balance,
		address: address,
	}
}

// SigLockedSingleOutputFromBytes unmarshals a SigLockedSingleOutput from a sequence of bytes.
func SigLockedSingleOutputFromBytes(bytes []byte) (output *SigLockedSingleOutput, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if output, err = SigLockedSingleOutputFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SigLockedSingleOutput from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SigLockedSingleOutputFromMarshalUtil unmarshals a SigLockedSingleOutput using a MarshalUtil (for easier unmarshaling).
func SigLockedSingleOutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (output *SigLockedSingleOutput, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse OutputType (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if OutputType(outputType) != SigLockedSingleOutputType {
		err = xerrors.Errorf("invalid OutputType (%X): %w", outputType, ErrParseBytesFailed)
		return
	}

	output = &SigLockedSingleOutput{}
	if output.balance, err = marshalUtil.ReadUint64(); err != nil {
		err = xerrors.Errorf("failed to parse balance (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if output.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Address (%v): %w", err, ErrParseBytesFailed)
		return
	}

	return
}

// ID returns the identifier of the Output that is used to address the Output in the UTXODAG.
func (s *SigLockedSingleOutput) ID() OutputID {
	s.idMutex.RLock()
	defer s.idMutex.RUnlock()

	return s.id
}

// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (s *SigLockedSingleOutput) SetID(outputID OutputID) {
	s.idMutex.Lock()
	defer s.idMutex.Unlock()

	s.id = outputID
}

// Type returns the type of the Output which allows us to generically handle Outputs of different types.
func (s *SigLockedSingleOutput) Type() OutputType {
	return SigLockedSingleOutputType
}

// Balances returns the funds that are associated with the Output.
func (s *SigLockedSingleOutput) Balances() *ColoredBalances {
	balances := NewColoredBalances(map[Color]uint64{
		ColorIOTA: s.balance,
	})

	return balances
}

// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the Output.
func (s *SigLockedSingleOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock) (unlockValid bool, err error) {
	signatureUnlockBlock, correctType := unlockBlock.(*SignatureUnlockBlock)
	if !correctType {
		err = xerrors.Errorf("UnlockBlock does not match expected OutputType: %w", ErrTransactionInvalid)
		return
	}

	if unlockValid, err = signatureUnlockBlock.AddressSignatureValid(s.address, tx.UnsignedBytes()); err != nil {
		err = xerrors.Errorf("failed to check signature: %w", err)
		return
	}

	return
}

// Address returns the Address that the Output is associated to.
func (s *SigLockedSingleOutput) Address() Address {
	return s.address
}

// Bytes returns a marshaled version of the Output.
func (s *SigLockedSingleOutput) Bytes() []byte {
	return s.ObjectStorageValue()
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (s *SigLockedSingleOutput) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (s *SigLockedSingleOutput) ObjectStorageKey() []byte {
	return s.ID().Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (s *SigLockedSingleOutput) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteByte(byte(SigLockedSingleOutputType)).
		WriteUint64(s.balance).
		WriteBytes(s.address.Bytes()).
		Bytes()
}

// String returns a human readable version of the Output.
func (s *SigLockedSingleOutput) String() string {
	return stringify.Struct("SigLockedSingleOutput",
		stringify.StructField("id", s.ID()),
		stringify.StructField("address", s.address),
		stringify.StructField("balance", s.balance),
	)
}

// code contract (make sure the type implements all required methods)
var _ Output = &SigLockedSingleOutput{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SigLockedColoredOutput ////////////////////////////////////////////////////////////////////////////////////////

// SigLockedColoredOutput is an Output that holds colored balances and that can be unlocked by providing a signature for
// an Address.
type SigLockedColoredOutput struct {
	id       OutputID
	idMutex  sync.RWMutex
	balances *ColoredBalances
	address  Address

	objectstorage.StorableObjectFlags
}

// NewSigLockedColoredOutput is the constructor for a SigLockedColoredOutput.
func NewSigLockedColoredOutput(balances *ColoredBalances, address Address) *SigLockedColoredOutput {
	return &SigLockedColoredOutput{
		balances: balances,
		address:  address,
	}
}

// SigLockedColoredOutputFromBytes unmarshals a SigLockedColoredOutput from a sequence of bytes.
func SigLockedColoredOutputFromBytes(bytes []byte) (output *SigLockedColoredOutput, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if output, err = SigLockedColoredOutputFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SigLockedColoredOutput from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SigLockedColoredOutputFromMarshalUtil unmarshals a SigLockedColoredOutput using a MarshalUtil (for easier unmarshaling).
func SigLockedColoredOutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (output *SigLockedColoredOutput, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse OutputType (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if OutputType(outputType) != SigLockedColoredOutputType {
		err = xerrors.Errorf("invalid OutputType (%X): %w", outputType, ErrParseBytesFailed)
		return
	}

	output = &SigLockedColoredOutput{}
	if output.balances, err = ColoredBalancesFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ColoredBalances: %w", err)
		return
	}
	if output.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Address (%v): %w", err, ErrParseBytesFailed)
		return
	}

	return
}

// ID returns the identifier of the Output that is used to address the Output in the UTXODAG.
func (s *SigLockedColoredOutput) ID() OutputID {
	s.idMutex.RLock()
	defer s.idMutex.RUnlock()

	return s.id
}

// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (s *SigLockedColoredOutput) SetID(outputID OutputID) {
	s.idMutex.Lock()
	defer s.idMutex.Unlock()

	s.id = outputID
}

// Type returns the type of the Output which allows us to generically handle Outputs of different types.
func (s *SigLockedColoredOutput) Type() OutputType {
	return SigLockedColoredOutputType
}

// Balances returns the funds that are associated with the Output.
func (s *SigLockedColoredOutput) Balances() *ColoredBalances {
	return s.balances
}

// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the Output.
func (s *SigLockedColoredOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock) (unlockValid bool, err error) {
	signatureUnlockBlock, correctType := unlockBlock.(*SignatureUnlockBlock)
	if !correctType {
		err = xerrors.Errorf("UnlockBlock does not match expected OutputType: %w", ErrTransactionInvalid)
		return
	}

	if unlockValid, err = signatureUnlockBlock.AddressSignatureValid(s.address, tx.UnsignedBytes()); err != nil {
		err = xerrors.Errorf("failed to check signature validity: %w", err)
		return
	}

	return
}

// Address returns the Address that the Output is associated to.
func (s *SigLockedColoredOutput) Address() Address {
	return s.address
}

// Bytes returns a marshaled version of the Output.
func (s *SigLockedColoredOutput) Bytes() []byte {
	return s.ObjectStorageValue()
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (s *SigLockedColoredOutput) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (s *SigLockedColoredOutput) ObjectStorageKey() []byte {
	return s.id.Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (s *SigLockedColoredOutput) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteByte(byte(SigLockedColoredOutputType)).
		WriteBytes(s.balances.Bytes()).
		WriteBytes(s.address.Bytes()).
		Bytes()
}

// String returns a human readable version of the Output.
func (s *SigLockedColoredOutput) String() string {
	return stringify.Struct("SigLockedColoredOutput",
		stringify.StructField("id", s.ID()),
		stringify.StructField("address", s.address),
		stringify.StructField("balances", s.balances),
	)
}

// code contract (make sure the type implements all required methods)
var _ Output = &SigLockedColoredOutput{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedOutput /////////////////////////////////////////////////////////////////////////////////////////////////

// CachedOutput is a wrapper for the generic CachedObject returned by the objectstorage that overrides the accessor
// methods with a type-casted one.
type CachedOutput struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedOutput) Retain() *CachedOutput {
	return &CachedOutput{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedOutput) Unwrap() Output {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(Output)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedOutput) Consume(consumer func(output Output), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(Output))
	}, forceRelease...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

// OutputMetadata contains additional Output information that are derived from the local perception of the node.
type OutputMetadata struct {
	id                      OutputID
	branchID                BranchID
	branchIDMutex           sync.RWMutex
	solid                   bool
	solidMutex              sync.RWMutex
	solidificationTime      time.Time
	solidificationTimeMutex sync.RWMutex
	consumerCount           int
	firstConsumer           TransactionID
	consumerMutex           sync.RWMutex
	preferred               bool
	preferredMutex          sync.RWMutex
	liked                   bool
	likedMutex              sync.RWMutex
	finalized               bool
	finalizedMutex          sync.RWMutex
	confirmed               bool
	confirmedMutex          sync.RWMutex
	rejected                bool
	rejectedMutex           sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewOutputMetadata creates a new empty OutputMetadata object.
func NewOutputMetadata(outputID OutputID) *OutputMetadata {
	return &OutputMetadata{
		id: outputID,
	}
}

// OutputMetadataFromBytes unmarshals an OutputMetadata object from a sequence of bytes.
func OutputMetadataFromBytes(bytes []byte) (outputMetadata *OutputMetadata, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if outputMetadata, err = OutputMetadataFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse OutputMetadata from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// OutputMetadataFromMarshalUtil unmarshals an OutputMetadata object using a MarshalUtil (for easier unmarshaling).
func OutputMetadataFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (outputMetadata *OutputMetadata, err error) {
	outputMetadata = &OutputMetadata{}
	if outputMetadata.id, err = OutputIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse OutputID: %w", err)
		return
	}
	if outputMetadata.branchID, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse BranchID: %w", err)
		return
	}
	if outputMetadata.solid, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse solid flag (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if outputMetadata.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		err = xerrors.Errorf("failed to parse solidification time (%v): %w", err, ErrParseBytesFailed)
		return
	}
	consumerCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse consumer count (%v): %w", err, ErrParseBytesFailed)
		return
	}
	outputMetadata.consumerCount = int(consumerCount)
	if outputMetadata.firstConsumer, err = TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse first consumer: %w", err)
		return
	}
	if outputMetadata.preferred, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse preferred flag (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if outputMetadata.liked, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse liked flag (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if outputMetadata.finalized, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse finalized flag (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if outputMetadata.confirmed, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse confirmed flag (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if outputMetadata.rejected, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse rejected flag (%v): %w", err, ErrParseBytesFailed)
		return
	}

	return
}

// OutputMetadataFromObjectStorage restores an OutputMetadata object that was stored in the ObjectStorage.
func OutputMetadataFromObjectStorage(key []byte, data []byte) (outputMetadata *OutputMetadata, err error) {
	if outputMetadata, _, err = OutputMetadataFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse OutputMetadata from bytes: %w", err)
		return
	}

	return
}

// ID returns the OutputID of the Output that the OutputMetadata belongs to.
func (o *OutputMetadata) ID() OutputID {
	return o.id
}

// BranchID returns the identifier of the Branch that the Output was booked in.
func (o *OutputMetadata) BranchID() BranchID {
	o.branchIDMutex.RLock()
	defer o.branchIDMutex.RUnlock()

	return o.branchID
}

// SetBranchID sets the identifier of the Branch that the Output was booked in.
func (o *OutputMetadata) SetBranchID(branchID BranchID) (modified bool) {
	o.branchIDMutex.RLock()
	if o.branchID == branchID {
		o.branchIDMutex.RUnlock()

		return
	}

	o.branchIDMutex.RUnlock()
	o.branchIDMutex.Lock()
	defer o.branchIDMutex.Unlock()

	if o.branchID == branchID {
		return
	}

	o.branchID = branchID
	o.SetModified()
	modified = true

	return
}

// Solid returns true if the Output has been marked as solid.
func (o *OutputMetadata) Solid() bool {
	o.solidMutex.RLock()
	defer o.solidMutex.RUnlock()

	return o.solid
}

// SetSolid updates the solid flag of the Output. It returns true if the solid flag was modified and updates the
// solidification time if the Output was marked as solid.
func (o *OutputMetadata) SetSolid(solid bool) (modified bool) {
	o.solidMutex.RLock()
	if o.solid != solid {
		o.solidMutex.RUnlock()

		o.solidMutex.Lock()
		if o.solid != solid {
			o.solid = solid
			if solid {
				o.solidificationTimeMutex.Lock()
				o.solidificationTime = time.Now()
				o.solidificationTimeMutex.Unlock()
			}

			o.SetModified()

			modified = true
		}
		o.solidMutex.Unlock()

	} else {
		o.solidMutex.RUnlock()
	}

	return
}

// SolidificationTime returns the time when the Output was marked as solid.
func (o *OutputMetadata) SolidificationTime() time.Time {
	o.solidificationTimeMutex.RLock()
	defer o.solidificationTimeMutex.RUnlock()

	return o.solidificationTime
}

// ConsumerCount returns the number of transactions that have spent the Output.
func (o *OutputMetadata) ConsumerCount() int {
	o.consumerMutex.RLock()
	defer o.consumerMutex.RUnlock()

	return o.consumerCount
}

// RegisterConsumer increases the consumer count of an Output and stores the first Consumer that was ever registered. It
// returns the previous consumer count.
func (o *OutputMetadata) RegisterConsumer(consumer TransactionID) (previousConsumerCount int) {
	o.consumerMutex.Lock()
	defer o.consumerMutex.Unlock()

	if previousConsumerCount = o.consumerCount; previousConsumerCount == 0 {
		o.firstConsumer = consumer
	}
	o.consumerCount++
	o.SetModified()

	return
}

// FirstConsumer returns the first TransactionID that ever spent the Output.
func (o *OutputMetadata) FirstConsumer() TransactionID {
	o.consumerMutex.RLock()
	defer o.consumerMutex.RUnlock()

	return o.firstConsumer
}

// Preferred returns true if the Output belongs to a preferred transaction.
func (o *OutputMetadata) Preferred() bool {
	o.preferredMutex.RLock()
	defer o.preferredMutex.RUnlock()

	return o.preferred
}

// SetPreferred updates the preferred flag.
func (o *OutputMetadata) SetPreferred(preferred bool) (modified bool) {
	o.preferredMutex.RLock()
	if o.preferred == preferred {
		o.preferredMutex.RUnlock()

		return
	}

	o.preferredMutex.RUnlock()
	o.preferredMutex.Lock()
	defer o.preferredMutex.Unlock()

	if o.preferred == preferred {
		return
	}

	o.preferred = preferred
	o.SetModified()
	modified = true

	return
}

// Liked returns true if the Output was marked as liked.
func (o *OutputMetadata) Liked() bool {
	o.likedMutex.RLock()
	defer o.likedMutex.RUnlock()

	return o.liked
}

// SetLiked modifies the liked flag. It returns true if the value has been modified.
func (o *OutputMetadata) SetLiked(liked bool) (modified bool) {
	o.likedMutex.RLock()
	if o.liked == liked {
		o.likedMutex.RUnlock()

		return
	}

	o.likedMutex.RUnlock()
	o.likedMutex.Lock()
	defer o.likedMutex.Unlock()

	if o.liked == liked {
		return
	}

	o.liked = liked
	o.SetModified()
	modified = true

	return
}

// Finalized returns true if the decision if the Output is preferred or not has been finalized by consensus already.
func (o *OutputMetadata) Finalized() bool {
	o.finalizedMutex.RLock()
	defer o.finalizedMutex.RUnlock()

	return o.finalized
}

// SetFinalized modifies the finalized flag. Finalized Outputs will not be forked when a conflict arrives later.
func (o *OutputMetadata) SetFinalized(finalized bool) (modified bool) {
	o.finalizedMutex.RLock()
	if o.finalized == finalized {
		o.finalizedMutex.RUnlock()

		return
	}

	o.finalizedMutex.RUnlock()
	o.finalizedMutex.Lock()
	defer o.finalizedMutex.Unlock()

	if o.finalized == finalized {
		return
	}

	o.finalized = finalized
	o.SetModified()
	modified = true

	return
}

// Confirmed returns true if the Output was marked as confirmed.
func (o *OutputMetadata) Confirmed() bool {
	o.confirmedMutex.RLock()
	defer o.confirmedMutex.RUnlock()

	return o.confirmed
}

// SetConfirmed modifies the confirmed flag. It returns true if the value has been updated.
func (o *OutputMetadata) SetConfirmed(confirmed bool) (modified bool) {
	o.confirmedMutex.RLock()
	if o.confirmed == confirmed {
		o.confirmedMutex.RUnlock()

		return
	}

	o.confirmedMutex.RUnlock()
	o.confirmedMutex.Lock()
	defer o.confirmedMutex.Unlock()

	if o.confirmed == confirmed {
		return
	}

	o.confirmed = confirmed
	o.SetModified()
	modified = true

	return
}

// Rejected returns true if the Output was marked as rejected.
func (o *OutputMetadata) Rejected() bool {
	o.rejectedMutex.RLock()
	defer o.rejectedMutex.RUnlock()

	return o.rejected
}

// SetRejected modifies the rejected flag. It returns true if the value has been updated.
func (o *OutputMetadata) SetRejected(rejected bool) (modified bool) {
	o.rejectedMutex.RLock()
	if o.rejected == rejected {
		o.rejectedMutex.RUnlock()

		return
	}

	o.rejectedMutex.RUnlock()
	o.rejectedMutex.Lock()
	defer o.rejectedMutex.Unlock()

	if o.rejected == rejected {
		return
	}

	o.rejected = rejected
	o.SetModified()
	modified = true

	return
}

// Bytes marshals the OutputMetadata into a sequence of bytes.
func (o *OutputMetadata) Bytes() []byte {
	return byteutils.ConcatBytes(o.ObjectStorageKey(), o.ObjectStorageValue())
}

// String returns a human readable version of the OutputMetadata.
func (o *OutputMetadata) String() string {
	return stringify.Struct("OutputMetadata",
		stringify.StructField("id", o.ID()),
		stringify.StructField("branchID", o.BranchID()),
		stringify.StructField("solid", o.Solid()),
		stringify.StructField("solidificationTime", o.SolidificationTime()),
		stringify.StructField("consumerCount", o.ConsumerCount()),
		stringify.StructField("firstConsumer", o.FirstConsumer()),
		stringify.StructField("preferred", o.Preferred()),
		stringify.StructField("liked", o.Liked()),
		stringify.StructField("finalized", o.Finalized()),
		stringify.StructField("confirmed", o.Confirmed()),
		stringify.StructField("rejected", o.Rejected()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (o *OutputMetadata) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (o *OutputMetadata) ObjectStorageKey() []byte {
	return o.id.Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (o *OutputMetadata) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteBytes(o.BranchID().Bytes()).
		WriteBool(o.Solid()).
		WriteTime(o.SolidificationTime()).
		WriteUint64(uint64(o.ConsumerCount())).
		WriteBytes(o.FirstConsumer().Bytes()).
		WriteBool(o.Preferred()).
		WriteBool(o.Liked()).
		WriteBool(o.Finalized()).
		WriteBool(o.Confirmed()).
		WriteBool(o.Rejected()).
		Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = &OutputMetadata{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedOutputMetadata /////////////////////////////////////////////////////////////////////////////////////////

// CachedOutputMetadata is a wrapper for the generic CachedObject returned by the objectstorage that overrides the
// accessor methods with a type-casted one.
type CachedOutputMetadata struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedOutputMetadata) Retain() *CachedOutputMetadata {
	return &CachedOutputMetadata{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedOutputMetadata) Unwrap() *OutputMetadata {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*OutputMetadata)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedOutputMetadata) Consume(consumer func(cachedOutputMetadata *OutputMetadata), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*OutputMetadata))
	}, forceRelease...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
