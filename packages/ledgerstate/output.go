package ledgerstate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/mr-tron/base58"
	"golang.org/x/xerrors"
)

// region Constraints for syntactical validation ///////////////////////////////////////////////////////////////////////

const (
	// MinOutputCount defines the minimum amount of Outputs in a Transaction.
	MinOutputCount = 1

	// MaxOutputCount defines the maximum amount of Outputs in a Transaction.
	MaxOutputCount = 127

	// MinOutputBalance defines the minimum balance per Output.
	MinOutputBalance = 1

	// MaxOutputBalance defines the maximum balance on an Output (the supply).
	MaxOutputBalance = 2779530283277761
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

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
const OutputIDLength = TransactionIDLength + marshalutil.Uint16Size

// OutputID is the data type that represents the identifier of an Output (which consists of a TransactionID and the
// index of the Output in the Transaction that created it).
type OutputID [OutputIDLength]byte

// EmptyOutputID represents the zero-value of an OutputID.
var EmptyOutputID OutputID

// NewOutputID is the constructor for the OutputID.
func NewOutputID(transactionID TransactionID, outputIndex uint16) (outputID OutputID) {
	if outputIndex >= MaxOutputCount {
		panic(fmt.Sprintf("output index exceeds threshold defined by MaxOutputCount (%d)", MaxOutputCount))
	}

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
		err = xerrors.Errorf("error while decoding base58 encoded OutputID (%v): %w", err, cerrors.ErrBase58DecodeFailed)
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
		err = xerrors.Errorf("failed to parse OutputID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(outputID[:], outputIDBytes)

	if outputID.OutputIndex() >= MaxOutputCount {
		err = xerrors.Errorf("output index exceeds threshold defined by MaxOutputCount (%d): %w", MaxOutputCount, cerrors.ErrParseBytesFailed)
		return
	}

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
	SetID(outputID OutputID) Output

	// Type returns the OutputType which allows us to generically handle Outputs of different types.
	Type() OutputType

	// Balances returns the funds that are associated with the Output.
	Balances() *ColoredBalances

	// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the
	// Output.
	UnlockValid(tx *Transaction, unlockBlock UnlockBlock) (bool, error)

	// Input returns an Input that references the Output.
	Input() Input

	// Clone creates a copy of the Output.
	Clone() Output

	// Bytes returns a marshaled version of the Output.
	Bytes() []byte

	// String returns a human readable version of the Output for debug purposes.
	String() string

	// Compare offers a comparator for Outputs which returns -1 if the other Output is bigger, 1 if it is smaller and 0
	// if they are the same.
	Compare(other Output) int

	// StorableObject makes Outputs storable in the ObjectStorage.
	objectstorage.StorableObject
}

// OutputFromBytes unmarshals an Output from a sequence of bytes.
func OutputFromBytes(bytes []byte) (output Output, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if output, err = OutputFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Output from MarshalUtil: %w", err)
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// OutputFromMarshalUtil unmarshals an Output using a MarshalUtil (for easier unmarshaling).
func OutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (output Output, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
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
		err = xerrors.Errorf("unsupported OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// OutputFromObjectStorage restores an Output that was stored in the ObjectStorage.
func OutputFromObjectStorage(key []byte, data []byte) (output objectstorage.StorableObject, err error) {
	if output, _, err = OutputFromBytes(data); err != nil {
		err = xerrors.Errorf("failed to parse Output from bytes: %w", err)
		return
	}

	outputID, _, err := OutputIDFromBytes(key)
	if err != nil {
		err = xerrors.Errorf("failed to parse OutputID from bytes: %w", err)
		return
	}
	output.(Output).SetID(outputID)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Outputs //////////////////////////////////////////////////////////////////////////////////////////////////////

// Outputs represents a list of Outputs that can be used in a Transaction.
type Outputs []Output

// NewOutputs returns a deterministically ordered collection of Outputs. It removes duplicates in the parameters and
// sorts the Outputs to ensure syntactical correctness.
func NewOutputs(optionalOutputs ...Output) (outputs Outputs) {
	seenOutputs := make(map[string]types.Empty)
	sortedOutputs := make([]struct {
		output           Output
		outputSerialized []byte
	}, 0)

	// filter duplicates (store marshaled version so we don't need to marshal a second time during sort)
	for _, output := range optionalOutputs {
		marshaledOutput := output.Bytes()
		marshaledOutputAsString := typeutils.BytesToString(marshaledOutput)

		if _, seenAlready := seenOutputs[marshaledOutputAsString]; seenAlready {
			continue
		}
		seenOutputs[marshaledOutputAsString] = types.Void

		sortedOutputs = append(sortedOutputs, struct {
			output           Output
			outputSerialized []byte
		}{output, marshaledOutput})
	}

	// sort outputs
	sort.Slice(sortedOutputs, func(i, j int) bool {
		return bytes.Compare(sortedOutputs[i].outputSerialized, sortedOutputs[j].outputSerialized) < 0
	})

	// create result
	outputs = make(Outputs, len(sortedOutputs))
	for i, sortedOutput := range sortedOutputs {
		outputs[i] = sortedOutput.output
	}

	if len(outputs) < MinOutputCount {
		panic(fmt.Sprintf("amount of Outputs (%d) failed to reach MinOutputCount (%d)", len(outputs), MinOutputCount))
	}
	if len(outputs) > MaxOutputCount {
		panic(fmt.Sprintf("amount of Outputs (%d) exceeds MaxOutputCount (%d)", len(outputs), MaxOutputCount))
	}

	return
}

// OutputsFromBytes unmarshals a collection of Outputs from a sequence of bytes.
func OutputsFromBytes(outputBytes []byte) (outputs Outputs, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(outputBytes)
	if outputs, err = OutputsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Outputs from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// OutputsFromMarshalUtil unmarshals a collection of Outputs using a MarshalUtil (for easier unmarshaling).
func OutputsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (outputs Outputs, err error) {
	outputsCount, err := marshalUtil.ReadUint16()
	if err != nil {
		err = xerrors.Errorf("failed to parse outputs count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if outputsCount < MinOutputCount {
		err = xerrors.Errorf("amount of Outputs (%d) failed to reach MinOutputCount (%d): %w", outputsCount, MinOutputCount, cerrors.ErrParseBytesFailed)
		return
	}
	if outputsCount > MaxOutputCount {
		err = xerrors.Errorf("amount of Outputs (%d) exceeds MaxOutputCount (%d): %w", outputsCount, MaxOutputCount, cerrors.ErrParseBytesFailed)
		return
	}

	var previousOutput Output
	parsedOutputs := make([]Output, outputsCount)
	for i := uint16(0); i < outputsCount; i++ {
		if parsedOutputs[i], err = OutputFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse Output from MarshalUtil: %w", err)
			return
		}

		if previousOutput != nil && previousOutput.Compare(parsedOutputs[i]) != -1 {
			err = xerrors.Errorf("order of Outputs is invalid: %w", cerrors.ErrParseBytesFailed)
			return
		}
		previousOutput = parsedOutputs[i]
	}

	outputs = NewOutputs(parsedOutputs...)

	return
}

// Inputs returns the Inputs that reference the Outputs.
func (o Outputs) Inputs() Inputs {
	inputs := make([]Input, len(o))
	for i, output := range o {
		inputs[i] = output.Input()
	}

	return NewInputs(inputs...)
}

// ByID returns a map of Outputs where the key is the OutputID.
func (o Outputs) ByID() (outputsByID OutputsByID) {
	outputsByID = make(OutputsByID)
	for _, output := range o {
		outputsByID[output.ID()] = output
	}

	return
}

// Clone creates a copy of the Outputs.
func (o Outputs) Clone() (clonedOutputs Outputs) {
	clonedOutputs = make(Outputs, len(o))
	for i, output := range o {
		clonedOutputs[i] = output.Clone()
	}

	return
}

// Bytes returns a marshaled version of the Outputs.
func (o Outputs) Bytes() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint16(uint16(len(o)))
	for _, output := range o {
		marshalUtil.WriteBytes(output.Bytes())
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the Outputs.
func (o Outputs) String() string {
	structBuilder := stringify.StructBuilder("Outputs")
	for i, output := range o {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), output))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsByID //////////////////////////////////////////////////////////////////////////////////////////////////

// OutputsByID represents a map of Outputs where every Output is stored with its corresponding OutputID as the key.
type OutputsByID map[OutputID]Output

// NewOutputsByID returns a map of Outputs where every Output is stored with its corresponding OutputID as the key.
func NewOutputsByID(optionalOutputs ...Output) (outputsByID OutputsByID) {
	outputsByID = make(OutputsByID)
	for _, optionalOutput := range optionalOutputs {
		outputsByID[optionalOutput.ID()] = optionalOutput
	}

	return
}

// Inputs returns the Inputs that reference the Outputs.
func (o OutputsByID) Inputs() Inputs {
	inputs := make([]Input, 0, len(o))
	for _, output := range o {
		inputs = append(inputs, output.Input())
	}

	return NewInputs(inputs...)
}

// Outputs returns a list of Outputs from the OutputsByID.
func (o OutputsByID) Outputs() Outputs {
	outputs := make([]Output, 0, len(o))
	for _, output := range o {
		outputs = append(outputs, output)
	}

	return NewOutputs(outputs...)
}

// Clone creates a copy of the OutputsByID.
func (o OutputsByID) Clone() (clonedOutputs OutputsByID) {
	clonedOutputs = make(OutputsByID)
	for id, output := range o {
		clonedOutputs[id] = output.Clone()
	}

	return
}

// String returns a human readable version of the OutputsByID.
func (o OutputsByID) String() string {
	structBuilder := stringify.StructBuilder("OutputsByID")
	for id, output := range o {
		structBuilder.AddField(stringify.StructField(id.String(), output))
	}

	return structBuilder.String()
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
		err = xerrors.Errorf("failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if OutputType(outputType) != SigLockedSingleOutputType {
		err = xerrors.Errorf("invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
		return
	}

	output = &SigLockedSingleOutput{}
	if output.balance, err = marshalUtil.ReadUint64(); err != nil {
		err = xerrors.Errorf("failed to parse balance (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if output.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Address (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	if output.balance < MinOutputBalance {
		err = xerrors.Errorf("balance (%d) is smaller than MinOutputBalance (%d): %w", output.balance, MinOutputBalance, cerrors.ErrParseBytesFailed)
		return
	}
	if output.balance > MaxOutputBalance {
		err = xerrors.Errorf("balance (%d) is bigger than MaxOutputBalance (%d): %w", output.balance, MaxOutputBalance, cerrors.ErrParseBytesFailed)
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
func (s *SigLockedSingleOutput) SetID(outputID OutputID) Output {
	s.idMutex.Lock()
	defer s.idMutex.Unlock()

	s.id = outputID

	return s
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
		err = xerrors.Errorf("UnlockBlock does not match expected OutputType: %w", cerrors.ErrParseBytesFailed)
		return
	}

	unlockValid = signatureUnlockBlock.AddressSignatureValid(s.address, tx.Essence().Bytes())

	return
}

// Address returns the Address that the Output is associated to.
func (s *SigLockedSingleOutput) Address() Address {
	return s.address
}

// Input returns an Input that references the Output.
func (s *SigLockedSingleOutput) Input() Input {
	if s.ID() == EmptyOutputID {
		panic("Outputs that haven't been assigned an ID yet cannot be converted to an Input")
	}

	return NewUTXOInput(s.ID())
}

// Clone creates a copy of the Output.
func (s *SigLockedSingleOutput) Clone() Output {
	return &SigLockedSingleOutput{
		id:      s.id,
		balance: s.balance,
		address: s.address.Clone(),
	}
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

// Compare offers a comparator for Outputs which returns -1 if the other Output is bigger, 1 if it is smaller and 0 if
// they are the same.
func (s *SigLockedSingleOutput) Compare(other Output) int {
	return bytes.Compare(s.Bytes(), other.Bytes())
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

// region SigLockedColoredOutput ///////////////////////////////////////////////////////////////////////////////////////

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
		err = xerrors.Errorf("failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if OutputType(outputType) != SigLockedColoredOutputType {
		err = xerrors.Errorf("invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
		return
	}

	output = &SigLockedColoredOutput{}
	if output.balances, err = ColoredBalancesFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ColoredBalances: %w", err)
		return
	}
	if output.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Address (%v): %w", err, cerrors.ErrParseBytesFailed)
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
func (s *SigLockedColoredOutput) SetID(outputID OutputID) Output {
	s.idMutex.Lock()
	defer s.idMutex.Unlock()

	s.id = outputID

	return s
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
		err = xerrors.Errorf("UnlockBlock does not match expected OutputType: %w", cerrors.ErrParseBytesFailed)
		return
	}

	unlockValid = signatureUnlockBlock.AddressSignatureValid(s.address, tx.Essence().Bytes())

	return
}

// Address returns the Address that the Output is associated to.
func (s *SigLockedColoredOutput) Address() Address {
	return s.address
}

// Input returns an Input that references the Output.
func (s *SigLockedColoredOutput) Input() Input {
	if s.ID() == EmptyOutputID {
		panic("Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	}

	return NewUTXOInput(s.ID())
}

// Clone creates a copy of the Output.
func (s *SigLockedColoredOutput) Clone() Output {
	return &SigLockedColoredOutput{
		id:       s.id,
		balances: s.balances.Clone(),
		address:  s.address.Clone(),
	}
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

// Compare offers a comparator for Outputs which returns -1 if the other Output is bigger, 1 if it is smaller and 0 if
// they are the same.
func (s *SigLockedColoredOutput) Compare(other Output) int {
	return bytes.Compare(s.Bytes(), other.Bytes())
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

// CachedOutput is a wrapper for the generic CachedObject returned by the object storage that overrides the accessor
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

// String returns a human readable version of the CachedOutput.
func (c *CachedOutput) String() string {
	return stringify.Struct("CachedOutput",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedOutputs ////////////////////////////////////////////////////////////////////////////////////////////////

// CachedOutputs represents a collection of CachedOutput objects.
type CachedOutputs []*CachedOutput

// Unwrap is the type-casted equivalent of Get. It returns a slice of unwrapped objects with the object being nil if it
// does not exist.
func (c CachedOutputs) Unwrap() (unwrappedOutputs []Output) {
	unwrappedOutputs = make([]Output, len(c))
	for i, cachedOutput := range c {
		untypedObject := cachedOutput.Get()
		if untypedObject == nil {
			continue
		}

		typedObject := untypedObject.(Output)
		if typedObject == nil || typedObject.IsDeleted() {
			continue
		}

		unwrappedOutputs[i] = typedObject
	}

	return
}

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (c CachedOutputs) Consume(consumer func(output Output), forceRelease ...bool) (consumed bool) {
	for _, cachedOutput := range c {
		consumed = cachedOutput.Consume(consumer, forceRelease...) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (c CachedOutputs) Release(force ...bool) {
	for _, cachedOutput := range c {
		cachedOutput.Release(force...)
	}
}

// String returns a human readable version of the CachedOutputs.
func (c CachedOutputs) String() string {
	structBuilder := stringify.StructBuilder("CachedOutputs")
	for i, cachedOutput := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedOutput))
	}

	return structBuilder.String()
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
	finalized               bool
	finalizedMutex          sync.RWMutex

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
		err = xerrors.Errorf("failed to parse solid flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if outputMetadata.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		err = xerrors.Errorf("failed to parse solidification time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	consumerCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse consumer count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	outputMetadata.consumerCount = int(consumerCount)
	if outputMetadata.firstConsumer, err = TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse first consumer: %w", err)
		return
	}
	if outputMetadata.finalized, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse finalized flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// OutputMetadataFromObjectStorage restores an OutputMetadata object that was stored in the ObjectStorage.
func OutputMetadataFromObjectStorage(key []byte, data []byte) (outputMetadata objectstorage.StorableObject, err error) {
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
	o.solidMutex.Lock()
	defer o.solidMutex.Unlock()

	if o.solid == solid {
		return
	}

	if solid {
		o.solidificationTimeMutex.Lock()
		o.solidificationTime = time.Now()
		o.solidificationTimeMutex.Unlock()
	}

	o.solid = solid
	o.SetModified()
	modified = true

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

// Finalized returns a boolean flag that indicates if the Transaction has been finalized regarding its decision of being
// included in the ledger state.
func (o *OutputMetadata) Finalized() (finalized bool) {
	o.finalizedMutex.RLock()
	defer o.finalizedMutex.RUnlock()

	return o.finalized
}

// SetFinalized updates the finalized flag of the Transaction. It returns true if the lazy booked flag was modified.
func (o *OutputMetadata) SetFinalized(finalized bool) (modified bool) {
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
		stringify.StructField("finalized", o.Finalized()),
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

// ObjectStorageValue marshals the OutputMetadata into a sequence of bytes. The ID is not serialized here as it is only
// used as a key in the ObjectStorage.
func (o *OutputMetadata) ObjectStorageValue() []byte {
	return marshalutil.New().
		Write(o.BranchID()).
		WriteBool(o.Solid()).
		WriteTime(o.SolidificationTime()).
		WriteUint64(uint64(o.ConsumerCount())).
		Write(o.FirstConsumer()).
		WriteBool(o.Finalized()).
		Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = &OutputMetadata{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsMetadata //////////////////////////////////////////////////////////////////////////////////////////////

// OutputsMetadata represents a list of OutputMetadata objects.
type OutputsMetadata []*OutputMetadata

// OutputIDs returns the OutputIDs of the Outputs in the list.
func (o OutputsMetadata) OutputIDs() (outputIDs []OutputID) {
	outputIDs = make([]OutputID, len(o))
	for i, outputMetadata := range o {
		outputIDs[i] = outputMetadata.ID()
	}

	return
}

// ConflictIDs returns the ConflictIDs that are the equivalent of the OutputIDs in the list.
func (o OutputsMetadata) ConflictIDs() (conflictIDs ConflictIDs) {
	conflictIDsSlice := make([]ConflictID, len(o))
	for i, input := range o {
		conflictIDsSlice[i] = NewConflictID(input.ID())
	}
	conflictIDs = NewConflictIDs(conflictIDsSlice...)

	return
}

// ByID returns a map of OutputsMetadata where the key is the OutputID.
func (o OutputsMetadata) ByID() (outputsMetadataByID OutputsMetadataByID) {
	outputsMetadataByID = make(OutputsMetadataByID)
	for _, outputMetadata := range o {
		outputsMetadataByID[outputMetadata.ID()] = outputMetadata
	}

	return
}

// String returns a human readable version of the OutputsMetadata.
func (o OutputsMetadata) String() string {
	structBuilder := stringify.StructBuilder("OutputsMetadata")
	for i, outputMetadata := range o {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), outputMetadata))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsMetadataByID //////////////////////////////////////////////////////////////////////////////////////////

// OutputsMetadataByID represents a map of OutputMetadatas where every OutputMetadata is stored with its corresponding
// OutputID as the key.
type OutputsMetadataByID map[OutputID]*OutputMetadata

// IDs returns the OutputIDs that are used as keys in the collection.
func (o OutputsMetadataByID) IDs() (outputIDs []OutputID) {
	outputIDs = make([]OutputID, 0, len(o))
	for outputID := range o {
		outputIDs = append(outputIDs, outputID)
	}

	return
}

// ConflictIDs turns the list of OutputMetadata objects into their corresponding ConflictIDs.
func (o OutputsMetadataByID) ConflictIDs() (conflictIDs ConflictIDs) {
	conflictIDsSlice := make([]ConflictID, 0, len(o))
	for inputID := range o {
		conflictIDsSlice = append(conflictIDsSlice, NewConflictID(inputID))
	}
	conflictIDs = NewConflictIDs(conflictIDsSlice...)

	return
}

// Filter returns the OutputsMetadataByID that are sharing a set membership with the given Inputs.
func (o OutputsMetadataByID) Filter(outputIDsToInclude []OutputID) (intersectionOfInputs OutputsMetadataByID) {
	intersectionOfInputs = make(OutputsMetadataByID)
	for _, outputID := range outputIDsToInclude {
		if output, exists := o[outputID]; exists {
			intersectionOfInputs[outputID] = output
		}
	}

	return
}

// String returns a human readable version of the OutputsMetadataByID.
func (o OutputsMetadataByID) String() string {
	structBuilder := stringify.StructBuilder("OutputsMetadataByID")
	for id, output := range o {
		structBuilder.AddField(stringify.StructField(id.String(), output))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedOutputMetadata /////////////////////////////////////////////////////////////////////////////////////////

// CachedOutputMetadata is a wrapper for the generic CachedObject returned by the object storage that overrides the
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
func (c *CachedOutputMetadata) Consume(consumer func(outputMetadata *OutputMetadata), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*OutputMetadata))
	}, forceRelease...)
}

// String returns a human readable version of the CachedOutputMetadata.
func (c *CachedOutputMetadata) String() string {
	return stringify.Struct("CachedOutputMetadata",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedOutputsMetadata ////////////////////////////////////////////////////////////////////////////////////////

// CachedOutputsMetadata represents a collection of CachedOutputMetadata objects.
type CachedOutputsMetadata []*CachedOutputMetadata

// Unwrap is the type-casted equivalent of Get. It returns a slice of unwrapped objects with the object being nil if it
// does not exist.
func (c CachedOutputsMetadata) Unwrap() (unwrappedOutputs []*OutputMetadata) {
	unwrappedOutputs = make([]*OutputMetadata, len(c))
	for i, cachedOutputMetadata := range c {
		untypedObject := cachedOutputMetadata.Get()
		if untypedObject == nil {
			continue
		}

		typedObject := untypedObject.(*OutputMetadata)
		if typedObject == nil || typedObject.IsDeleted() {
			continue
		}

		unwrappedOutputs[i] = typedObject
	}

	return
}

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (c CachedOutputsMetadata) Consume(consumer func(consumer *OutputMetadata), forceRelease ...bool) (consumed bool) {
	for _, cachedOutputMetadata := range c {
		consumed = cachedOutputMetadata.Consume(consumer, forceRelease...) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (c CachedOutputsMetadata) Release(force ...bool) {
	for _, cachedOutputMetadata := range c {
		cachedOutputMetadata.Release(force...)
	}
}

// String returns a human readable version of the CachedOutputsMetadata.
func (c CachedOutputsMetadata) String() string {
	structBuilder := stringify.StructBuilder("CachedOutputsMetadata")
	for i, cachedOutputMetadata := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedOutputMetadata))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
