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
	"golang.org/x/crypto/blake2b"
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

	// ChainOutputType represents an Output which makes a chain with optional governance
	ChainOutputType

	// ExtendedLockedOutputType represents an Output which extends SigLockedColoredOutput with alias locking and fallback
	ExtendedLockedOutputType
)

// String returns a human readable representation of the OutputType.
func (o OutputType) String() string {
	return [...]string{
		"SigLockedSingleOutputType",
		"SigLockedColoredOutputType",
		"ChainOutputType",
		"ExtendedLockedOutputType",
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

	// Address returns the address that is associated to the output.
	Address() Address

	// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the
	// Output.
	UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (bool, error)

	// UpdateMintingColor replaces the ColorMint in the balances of the Output with the hash of the OutputID. It returns a
	// copy of the original Output with the modified balances.
	UpdateMintingColor() Output

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
	case ChainOutputType:
		if output, err = ChainOutputFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse ChainOutput: %w", err)
			return
		}
	case ExtendedLockedOutputType:
		if output, err = ExtendedOutputFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse ExtendedOutput: %w", err)
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
func (s *SigLockedSingleOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (unlockValid bool, err error) {
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

func (s *SigLockedSingleOutput) UpdateMintingColor() Output {
	return s
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
func (s *SigLockedColoredOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (unlockValid bool, err error) {
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

// UpdateMintingColor replaces the ColorMint in the balances of the Output with the hash of the OutputID. It returns a
// copy of the original Output with the modified balances.
func (s *SigLockedColoredOutput) UpdateMintingColor() (updatedOutput Output) {
	coloredBalances := s.Balances().Map()
	if mintedCoins, mintedCoinsExist := coloredBalances[ColorMint]; mintedCoinsExist {
		delete(coloredBalances, ColorMint)
		coloredBalances[Color(blake2b.Sum256(s.ID().Bytes()))] = mintedCoins
	}
	updatedOutput = NewSigLockedColoredOutput(NewColoredBalances(coloredBalances), s.Address())
	updatedOutput.SetID(s.ID())

	return
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

// region ChainOutput ///////////////////////////////////////////////////////////////////////////////////////

const DustThresholdChainOutputIOTA = uint64(100) // TODO protocol-wide dust threshold

// MaxOutputPayloadSize size limit on the data payload in the output.
const MaxOutputPayloadSize = 4 * 1024

// flags use to compress serialized bytes
const (
	flagChainOutputGovernanceUpdate = 0x01
	flagChainOutputGovernanceSet    = 0x02
	flagChainOutputStateDataPresent = 0x04
)

// ChainOutput represents output which defines as AliasAddress.
// It can only be used in a chained manner
type ChainOutput struct {
	// common for all outputs
	outputId      OutputID
	outputIdMutex sync.RWMutex
	balances      ColoredBalances

	// aliasAddress becomes immutable after created for a lifetime. It is returned as Address()
	aliasAddress AliasAddress
	// address which controls the state and state data
	// It can only be changed by governing entity, if set. Otherwise it is self governed.
	// It should be an address unlocked by signature, not AliasAddress
	stateAddress Address
	// optional state metadata. nil means absent
	stateData []byte
	// if the ChainOutput is chained in the transaction, the flags states if it is updating state or governance data.
	// unlock validation of the corresponding input depends on it.
	// The flag is used to prevent a need to check signature each time when checking unlocking mode
	isGovernanceUpdate bool
	// governance address if set. It can be any address, unlocked by signature of alias address. Nil means self governed
	governingAddress Address

	objectstorage.StorableObjectFlags
}

// NewChainOutputMint creates new ChainOutput as minting output, i.e. the one which does not contain corresponding input.
func NewChainOutputMint(balances map[Color]uint64, stateAddr Address) (*ChainOutput, error) {
	if !IsAboveDustThreshold(balances) {
		return nil, xerrors.New("ChainOutput: colored balances should not be empty")
	}
	if stateAddr == nil || stateAddr.Type() == AliasAddressType {
		return nil, xerrors.New("ChainOutput: enforcing mandatory state address must be backed by a private key, can't be an alias")
	}
	ret := &ChainOutput{
		balances:     *NewColoredBalances(balances),
		stateAddress: stateAddr,
	}
	if err := ret.checkValidity(); err != nil {
		return nil, err
	}
	return ret, nil
}

// NewChainOutputNext creates new ChainOutput as state transition from the previous one
func (c *ChainOutput) NewChainOutputNext(governanceUpdate ...bool) *ChainOutput {
	ret := c.clone()
	ret.aliasAddress = *c.GetAliasAddress()
	ret.isGovernanceUpdate = false
	if len(governanceUpdate) > 0 {
		ret.isGovernanceUpdate = governanceUpdate[0]
	}
	return ret
}

// ChainOutputFromMarshalUtil unmarshals a ChainOutput using a MarshalUtil (for easier unmarshaling).
func ChainOutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (*ChainOutput, error) {
	var ret *ChainOutput
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, xerrors.Errorf("ChainOutput: failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if OutputType(outputType) != ChainOutputType {
		return nil, xerrors.Errorf("ChainOutput: invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
	}
	ret = &ChainOutput{}
	flags, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, xerrors.Errorf("ChainOutput: failed to parse ChainOutput flags (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	ret.isGovernanceUpdate = flags&flagChainOutputGovernanceUpdate != 0
	if ret.aliasAddress.IsNil() {
		addr, err := AliasAddressFromMarshalUtil(marshalUtil)
		if err != nil {
			return nil, xerrors.Errorf("ChainOutput: failed to parse alias address (%v): %w", err, cerrors.ErrParseBytesFailed)
		}
		ret.aliasAddress = *addr
	}
	cb, err := ColoredBalancesFromMarshalUtil(marshalUtil)
	if err != nil {
		return nil, xerrors.Errorf("ChainOutput: failed to parse colored balances: %w", err)
	}
	ret.balances = *cb
	ret.stateAddress, err = AddressFromMarshalUtil(marshalUtil)
	if err != nil {
		return nil, xerrors.Errorf("ChainOutput: failed to parse state address (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if flags&flagChainOutputStateDataPresent != 0 {
		size, err := marshalUtil.ReadUint16()
		if err != nil {
			return nil, xerrors.Errorf("ChainOutput: failed to parse state data size: %w", err)
		}
		ret.stateData, err = marshalUtil.ReadBytes(int(size))
		if err != nil {
			return nil, xerrors.Errorf("ChainOutput: failed to parse state data: %w", err)
		}
	}
	if flags&flagChainOutputGovernanceSet != 0 {
		ret.governingAddress, err = AddressFromMarshalUtil(marshalUtil)
		if err != nil {
			return nil, xerrors.Errorf("ChainOutput: failed to parse governing address (%v): %w", err, cerrors.ErrParseBytesFailed)
		}
	}
	if err := ret.checkValidity(); err != nil {
		return nil, err
	}
	return ret, nil
}

// SetBalances sets colored balances of the output
func (c *ChainOutput) SetBalances(balances map[Color]uint64) error {
	if !IsAboveDustThreshold(balances) {
		return xerrors.New("ChainOutput: balances are less than dust threshold")
	}
	c.balances = *NewColoredBalances(balances)
	return nil
}

// GetAliasAddress calculates new ID if it is a minting output. Otherwise it takes stored value
func (c *ChainOutput) GetAliasAddress() *AliasAddress {
	if c.aliasAddress.IsNil() {
		return NewAliasAddress(c.ID().Bytes())
	}
	return &c.aliasAddress
}

// IsSelfGoverned return if
func (c *ChainOutput) IsSelfGoverned() bool {
	return c.governingAddress == nil
}

// GetStateAddress return state controlling address
func (c *ChainOutput) GetStateAddress() Address {
	return c.stateAddress
}

// SetStateAddress sets the state controlling address
func (c *ChainOutput) SetStateAddress(addr Address) error {
	if addr == nil || addr.Type() == AliasAddressType {
		return xerrors.New("ChainOutput: mandatory state address cannot be c AliasAddress")
	}
	c.stateAddress = addr
	return nil
}

// SetGoverningAddress sets the governing address or nil for self-giverning
func (c *ChainOutput) SetGoverningAddress(addr Address) {
	if addr.Array() == c.stateAddress.Array() {
		addr = nil // self governing
	}
	c.governingAddress = addr
}

// GetGoverningAddress return governing address. If self-governed, it is the same as state controlling address
func (c *ChainOutput) GetGoverningAddress() Address {
	if c.IsSelfGoverned() {
		return c.stateAddress
	}
	return c.governingAddress
}

// SetStateData sets state data
func (c *ChainOutput) SetStateData(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return xerrors.New("ChainOutput: state data too big")
	}
	c.stateData = make([]byte, len(data))
	copy(c.stateData, data)
	return nil
}

// GetStatData gets the state data
func (c *ChainOutput) GetStateData() []byte {
	return c.stateData
}

// Clone clones the structure
func (c *ChainOutput) Clone() Output {
	return c.clone()
}

func (c *ChainOutput) clone() *ChainOutput {
	c.mustValidate()
	ret := &ChainOutput{
		outputId:     c.outputId,
		balances:     *c.balances.Clone(),
		aliasAddress: c.aliasAddress,
		stateAddress: c.stateAddress.Clone(),
		stateData:    make([]byte, len(c.stateData)),
	}
	if c.governingAddress != nil {
		ret.governingAddress = c.governingAddress.Clone()
	}
	copy(ret.stateData, c.stateData)
	ret.mustValidate()
	return ret
}

// ID is the ID of the output
func (c *ChainOutput) ID() OutputID {
	c.outputIdMutex.RLock()
	defer c.outputIdMutex.RUnlock()

	return c.outputId
}

// SetID set the output ID after unmarshalling
func (c *ChainOutput) SetID(outputID OutputID) Output {
	c.outputIdMutex.Lock()
	defer c.outputIdMutex.Unlock()

	c.outputId = outputID
	return c
}

// Type return the type of the output
func (c *ChainOutput) Type() OutputType {
	return ChainOutputType
}

// Balances return colored balances of the output
func (c *ChainOutput) Balances() *ColoredBalances {
	return &c.balances
}

// Address ChainOutput is searchable in the ledger through its AliasAddress
func (c *ChainOutput) Address() Address {
	return c.GetAliasAddress()
}

// Input makes input from the output
func (c *ChainOutput) Input() Input {
	if c.ID() == EmptyOutputID {
		panic("ChainOutput: Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	}

	return NewUTXOInput(c.ID())
}

// Bytes serialized form
func (c *ChainOutput) Bytes() []byte {
	return c.ObjectStorageValue()
}

// String human readable form
func (c *ChainOutput) String() string {
	ret := "ChainOutput:\n"
	ret += fmt.Sprintf("   address: %s\n", c.Address())
	ret += fmt.Sprintf("   outputId: %s\n", c.ID())
	ret += fmt.Sprintf("   balance: %s\n", c.balances.String())
	ret += fmt.Sprintf("   stateAddress: %s\n", c.stateAddress)
	ret += fmt.Sprintf("   stateMetadataSize: %d\n", len(c.stateData))
	ret += fmt.Sprintf("   governingAddress (self-governed=%v): %s\n", c.IsSelfGoverned(), c.GetGoverningAddress())
	return ret
}

// Compare the two outputs
func (c *ChainOutput) Compare(other Output) int {
	return bytes.Compare(c.Bytes(), other.Bytes())
}

// Update is disabled
func (c *ChainOutput) Update(other objectstorage.StorableObject) {
	panic("ChainOutput: storage object updates disabled")
}

// ObjectStorageKey a key
func (c *ChainOutput) ObjectStorageKey() []byte {
	return c.ID().Bytes()
}

// ObjectStorageValue binary form
func (c *ChainOutput) ObjectStorageValue() []byte {
	flags := c.mustFlags()
	ret := marshalutil.New().
		WriteByte(byte(ChainOutputType)).
		WriteByte(flags).
		WriteBytes(c.aliasAddress.Bytes()).
		WriteBytes(c.balances.Bytes()).
		WriteBytes(c.stateAddress.Bytes())
	if flags&flagChainOutputStateDataPresent != 0 {
		ret.WriteUint16(uint16(len(c.stateData))).
			WriteBytes(c.stateData)
	}
	if flags&flagChainOutputGovernanceSet != 0 {
		ret.WriteBytes(c.governingAddress.Bytes())
	}
	return ret.Bytes()
}

// UnlockValid check unlock and validates chain
func (c *ChainOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (bool, error) {
	switch blk := unlockBlock.(type) {
	case *SignatureUnlockBlock:
		stateUnlocked, governanceUnlocked, err := c.unlockedBySignature(tx, blk)

		return stateUnlocked || governanceUnlocked, err

	case *AliasUnlockBlock:
		// state cannot be unlocked by alias reference, so only checking governance mode
		return c.unlockedGovernanceByAliasIndex(tx, blk.ChainInputIndex(), inputs)
	}
	return false, xerrors.New("unsupported unlock block type")
}

func (c *ChainOutput) UpdateMintingColor() Output {
	coloredBalances := c.Balances().Map()
	if mintedCoins, mintedCoinsExist := coloredBalances[ColorMint]; mintedCoinsExist {
		delete(coloredBalances, ColorMint)
		coloredBalances[Color(blake2b.Sum256(c.ID().Bytes()))] = mintedCoins
	}
	updatedOutput := c.clone()
	_ = updatedOutput.SetBalances(coloredBalances)
	updatedOutput.SetID(c.ID())

	return updatedOutput
}

func (c *ChainOutput) checkValidity() error {
	if !IsAboveDustThreshold(c.balances.Map()) {
		return xerrors.New("ChainOutput: balances are below dust threshold")
	}
	if c.stateAddress == nil {
		return xerrors.New("ChainOutput: state address must not be nil")
	}
	if len(c.stateData) > MaxOutputPayloadSize {
		return xerrors.New("ChainOutput: state data too big")
	}
	return nil
}

func (c *ChainOutput) mustValidate() {
	if err := c.checkValidity(); err != nil {
		panic(err)
	}
}

func (c *ChainOutput) mustFlags() byte {
	c.mustValidate()
	var ret byte
	if c.isGovernanceUpdate {
		ret |= flagChainOutputGovernanceUpdate
	}
	if len(c.stateData) > 0 {
		ret |= flagChainOutputStateDataPresent
	}
	if c.governingAddress != nil {
		ret |= flagChainOutputGovernanceSet
	}
	return ret
}

func (c *ChainOutput) findChainedOutput(tx *Transaction) (*ChainOutput, error) {
	var ret *ChainOutput
	aliasAddress := c.GetAliasAddress()
	for _, out := range tx.Essence().Outputs() {
		if out.Type() != ChainOutputType {
			continue
		}
		outAlias := out.(*ChainOutput)
		if !aliasAddress.Equals(outAlias.GetAliasAddress()) {
			continue
		}
		if ret != nil {
			return nil, xerrors.Errorf("duplicated alias output: %s", aliasAddress.String())
		}
		ret = outAlias
	}
	return ret, nil
}

// unlockedBySig only possible unlocked for both state and governance when self-governed
func (c *ChainOutput) unlockedBySig(tx *Transaction, sigBlock *SignatureUnlockBlock) (bool, bool) {
	stateSigValid := sigBlock.signature.AddressSignatureValid(c.stateAddress, tx.Essence().Bytes())
	if c.IsSelfGoverned() {
		return stateSigValid, stateSigValid
	}
	if stateSigValid {
		return true, false
	}
	return false, sigBlock.signature.AddressSignatureValid(c.governingAddress, tx.Essence().Bytes())
}

func equalColoredBalance(b1, b2 ColoredBalances) bool {
	allColors := make(map[Color]bool)
	b1.ForEach(func(col Color, bal uint64) bool {
		allColors[col] = true
		return true
	})
	b2.ForEach(func(col Color, bal uint64) bool {
		allColors[col] = true
		return true
	})
	for col := range allColors {
		v1, ok1 := b1.Get(col)
		v2, ok2 := b2.Get(col)
		if ok1 != ok2 || v1 != v2 {
			return false
		}
	}
	return true
}

func IsAboveDustThreshold(m map[Color]uint64) bool {
	if iotas, ok := m[ColorIOTA]; ok && iotas >= DustThresholdChainOutputIOTA {
		return true
	}
	return false
}

func isExactMinimum(b ColoredBalances) bool {
	bals := b.Map()
	if len(bals) != 1 {
		return false
	}
	bal, ok := bals[ColorIOTA]
	if !ok || bal != DustThresholdChainOutputIOTA {
		return false
	}
	return true
}

// validateStateTransition checks if part controlled by state address is valid
func (c *ChainOutput) validateStateTransition(chained *ChainOutput, unlockedState bool) error {
	if unlockedState {
		if chained.isGovernanceUpdate {
			return xerrors.New("ChainOutput: wrong unlock for state update")
		}
		if !IsAboveDustThreshold(chained.balances.Map()) {
			return xerrors.New("ChainOutput: tokens are below dust threshold")
		}
		return nil
	}
	// locked: should not modify state data nor tokens
	if !bytes.Equal(c.stateData, chained.stateData) {
		return xerrors.New("ChainOutput: state data is locked for modification")
	}
	if !equalColoredBalance(c.balances, chained.balances) {
		return xerrors.New("ChainOutput: tokens are locked for modification")
	}

	return nil
}

// validateGovernanceChange checks if the parte controlled by the governing party is valid
func (c *ChainOutput) validateGovernanceChange(chained *ChainOutput, unlockedGovernance bool) error {
	if unlockedGovernance {
		return nil
	}
	if c.isGovernanceUpdate {
		return xerrors.New("ChainOutput: wrong governance update flag")
	}
	// locked: must not modify governance data
	if bytes.Compare(c.stateAddress.Bytes(), chained.stateAddress.Bytes()) != 0 {
		return xerrors.New("ChainOutput: state address is locked for modification")
	}
	if c.IsSelfGoverned() != chained.IsSelfGoverned() {
		return xerrors.New("ChainOutput: governing address is locked for modification")
	}
	if !c.IsSelfGoverned() {
		if bytes.Compare(c.governingAddress.Bytes(), chained.governingAddress.Bytes()) != 0 {
			return xerrors.New("ChainOutput: governing address is locked for modification")
		}
	}
	return nil
}

// validateDestroyTransition check validity if input is not chained (destroyed)
func (c *ChainOutput) validateDestroyTransition(unlockedGovernance bool) error {
	if !unlockedGovernance {
		return xerrors.New("ChainOutput: didn't find chained output and alias is not unlocked to be destroyed.")
	}
	if !isExactMinimum(c.balances) {
		return xerrors.New("ChainOutput: didn't find chained output and there are more tokens then upper limit for alias destruction")
	}
	return nil
}

func (c *ChainOutput) validateTransition(tx *Transaction, unlockedState, unlockedGovernance bool) error {
	chained, err := c.findChainedOutput(tx)
	if err != nil {
		return err
	}

	if chained != nil {
		if !c.GetAliasAddress().Equals(c.GetAliasAddress()) {
			return xerrors.New("chain alias address can't be modified")
		}
		if err := c.validateStateTransition(chained, unlockedState); err != nil {
			return err
		}
		if err := c.validateGovernanceChange(chained, unlockedGovernance); err != nil {
			return err
		}
	} else {
		// no chained output found. Alias is being destroyed?
		if err := c.validateDestroyTransition(unlockedGovernance); err != nil {
			return err
		}
	}
	return nil
}

func (c *ChainOutput) unlockedBySignature(tx *Transaction, sigBlock *SignatureUnlockBlock) (bool, bool, error) {
	unlockedState, unlockedGovernance := c.unlockedBySig(tx, sigBlock)
	if !unlockedState && !unlockedGovernance {
		return false, false, nil
	}
	if err := c.validateTransition(tx, unlockedState, unlockedGovernance); err != nil {
		return false, false, nil
	}
	return unlockedState, unlockedGovernance, nil
}

// unlockedGovernanceByAliasIndex unlock one step of alias dereference
func (c *ChainOutput) unlockedGovernanceByAliasIndex(tx *Transaction, refIndex uint16, inputs []Output) (bool, error) {
	if c.IsSelfGoverned() {
		return false, xerrors.New("ChainOutput: self-governing alias output can't be unlocked by alias reference")
	}
	if c.governingAddress.Type() != AliasAddressType {
		return false, xerrors.New("ChainOutput: expected governing address of AliasAddress type")
	}
	if int(refIndex) > len(inputs) {
		return false, xerrors.New("ChainOutput: wrong alias reference index")
	}
	refInput, ok := inputs[refIndex].(*ChainOutput)
	if !ok {
		return false, xerrors.New("ChainOutput: the referenced output is not of ChainOutput type")
	}
	if !refInput.GetAliasAddress().Equals(c.governingAddress.(*AliasAddress)) {
		return false, xerrors.New("ChainOutput: wrong alias reference address")
	}
	return !refInput.IsUnlockedForGovernanceUpdate(tx), nil
}

func (c *ChainOutput) IsUnlockedForGovernanceUpdate(tx *Transaction) bool {
	chained, err := c.findChainedOutput(tx)
	if err != nil {
		return false
	}
	return chained.isGovernanceUpdate
}

// code contract (make sure the type implements all required methods)
var _ Output = &ChainOutput{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ExtendedLockedOutput /////////////////////////////////////////////////////////////////////////////////////////////////

// ExtendedLockedOutput is an Extension of SigLockedColoredOutput. If extended options not enabled,
// it behaves as SigLockedColoredOutput.
// In addition it has options:
// - fallback address and fallback timeout
// - can be unlocked by AliasUnlockBlock (if address is of AliasAddress type)
// - can be time locked until deadline
// - data payload for arbitrary metadata (size limits apply)
type ExtendedLockedOutput struct {
	id       OutputID
	idMutex  sync.RWMutex
	balances *ColoredBalances
	address  Address // any address type

	// optional part
	// Fallback address after timeout. If nil, fallback action not set
	fallbackAddress Address
	// fallback deadline in Unix seconds. The deadline is calculated relative to the tx timestamo
	fallbackDeadline uint32

	// Deadline since when output can be unlocked. Unix seconds
	timelock uint32

	// any attached data (subject to size limits)
	payload []byte

	objectstorage.StorableObjectFlags
}

const (
	flagExtendedLockedOutputFallbackPresent = 0x01
	flagExtendedLockedOutputTimeLockPresent = 0x02
	flagExtendedLockedOutputPayloadPresent  = 0x04
)

// ExtendedLockedOutput is the constructor for a ExtendedLockedOutput.
func NewExtendedLockedOutput(balances map[Color]uint64, address Address) *ExtendedLockedOutput {
	return &ExtendedLockedOutput{
		balances: NewColoredBalances(balances),
		address:  address.Clone(),
	}
}

func (o *ExtendedLockedOutput) WithFallbackOptions(addr Address, deadline uint32) *ExtendedLockedOutput {
	o.fallbackAddress = addr.Clone()
	o.fallbackDeadline = deadline
	return o
}

func (o *ExtendedLockedOutput) WithTimeLock(timelock uint32) *ExtendedLockedOutput {
	o.timelock = timelock
	return o
}

func (o *ExtendedLockedOutput) SetPayload(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return xerrors.Errorf("ExtendedLockedOutput: data payload size (%d bytes) is bigger than maximum allowed (%d bytes)", len(data), MaxOutputPayloadSize)
	}
	o.payload = make([]byte, len(data))
	copy(o.payload, data)
	return nil
}

// ExtendedOutputFromBytes unmarshals a ExtendedLockedOutput from a sequence of bytes.
func ExtendedOutputFromBytes(bytes []byte) (output *ExtendedLockedOutput, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if output, err = ExtendedOutputFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ExtendedLockedOutput from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ExtendedOutputFromMarshalUtil unmarshals a ExtendedLockedOutput using a MarshalUtil (for easier unmarshaling).
func ExtendedOutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (output *ExtendedLockedOutput, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if OutputType(outputType) != ExtendedLockedOutputType {
		err = xerrors.Errorf("invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
		return
	}

	output = &ExtendedLockedOutput{}
	if output.balances, err = ColoredBalancesFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ColoredBalances: %w", err)
		return
	}
	if output.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Address (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	var flags byte
	if flags, err = marshalUtil.ReadByte(); err != nil {
		err = xerrors.Errorf("failed to parse flags (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if flagExtendedLockedOutputFallbackPresent&flags != 0 {
		if output.fallbackAddress, err = AddressFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse fallbackAddress (%v): %w", err, cerrors.ErrParseBytesFailed)
			return
		}
		if output.fallbackDeadline, err = marshalUtil.ReadUint32(); err != nil {
			err = xerrors.Errorf("failed to parse fallbackTimeout (%v): %w", err, cerrors.ErrParseBytesFailed)
			return
		}
	}
	if flagExtendedLockedOutputTimeLockPresent&flags != 0 {
		if output.timelock, err = marshalUtil.ReadUint32(); err != nil {
			err = xerrors.Errorf("failed to parse timelock (%v): %w", err, cerrors.ErrParseBytesFailed)
			return
		}
	}
	if flagExtendedLockedOutputPayloadPresent&flags != 0 {
		var size uint16
		size, err = marshalUtil.ReadUint16()
		if err != nil {
			err = xerrors.Errorf("failed to parse payload size (%v): %w", err, cerrors.ErrParseBytesFailed)
			return
		}
		output.payload, err = marshalUtil.ReadBytes(int(size))
		if err != nil {
			err = xerrors.Errorf("failed to parse payload (%v): %w", err, cerrors.ErrParseBytesFailed)
			return
		}
	}
	return
}

func (o *ExtendedLockedOutput) compressFlags() byte {
	var ret byte
	if o.fallbackAddress != nil {
		ret |= flagExtendedLockedOutputFallbackPresent
	}
	if o.timelock > 0 {
		ret |= flagExtendedLockedOutputTimeLockPresent
	}
	if len(o.payload) > 0 {
		ret |= flagExtendedLockedOutputPayloadPresent
	}
	return ret
}

// ID returns the identifier of the Output that is used to address the Output in the UTXODAG.
func (o *ExtendedLockedOutput) ID() OutputID {
	o.idMutex.RLock()
	defer o.idMutex.RUnlock()

	return o.id
}

// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (o *ExtendedLockedOutput) SetID(outputID OutputID) Output {
	o.idMutex.Lock()
	defer o.idMutex.Unlock()

	o.id = outputID

	return o
}

// Type returns the type of the Output which allows us to generically handle Outputs of different types.
func (o *ExtendedLockedOutput) Type() OutputType {
	return ExtendedLockedOutputType
}

// Balances returns the funds that are associated with the Output.
func (o *ExtendedLockedOutput) Balances() *ColoredBalances {
	return o.balances
}

// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the Output.
func (o *ExtendedLockedOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (unlockValid bool, err error) {
	if tx.Essence().Timestamp().Before(time.Unix(int64(o.timelock), 0)) {
		// can't be unlocked yet
		return false, nil
	}
	var addr Address
	if o.fallbackAddress == nil {
		// if fallback option is not set, the output can be unlocked by the main address
		addr = o.address
	} else {
		// fallback option is set
		// until fallback deadline the output can be unlocked by main address.
		// after fallback deadline it can be unlocked by fallback address
		if tx.Essence().Timestamp().Before(time.Unix(int64(o.fallbackDeadline), 0)) {
			addr = o.address
		} else {
			addr = o.fallbackAddress
		}
	}

	switch blk := unlockBlock.(type) {
	case *SignatureUnlockBlock:
		// unlocking by signature
		unlockValid = blk.AddressSignatureValid(addr, tx.Essence().Bytes())

	case *AliasUnlockBlock:
		// unlocking by alias reference
		refAliasOutput, isAlias := inputs[blk.ChainInputIndex()].(*ChainOutput)
		if !isAlias {
			return false, xerrors.New("ExtendedLockedOutput: referenced input must be ChainOutput")
		}
		if !addr.Equals(refAliasOutput.GetAliasAddress()) {
			return false, xerrors.New("ExtendedLockedOutput: wrong alias referenced")
		}
		unlockValid = refAliasOutput.IsSelfGoverned() || !refAliasOutput.IsUnlockedForGovernanceUpdate(tx)

	default:
		err = xerrors.Errorf("ExtendedLockedOutput: unsupported unlock block type: %w", cerrors.ErrParseBytesFailed)
	}
	return
}

// Address returns the Address that the Output is associated to.
func (o *ExtendedLockedOutput) Address() Address {
	return o.address
}

// Input returns an Input that references the Output.
func (o *ExtendedLockedOutput) Input() Input {
	if o.ID() == EmptyOutputID {
		panic("ExtendedLockedOutput: Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	}

	return NewUTXOInput(o.ID())
}

// Clone creates a copy of the Output.
func (o *ExtendedLockedOutput) Clone() Output {
	return &ExtendedLockedOutput{
		id:       o.id,
		balances: o.balances.Clone(),
		address:  o.address.Clone(),
	}
}

// UpdateMintingColor replaces the ColorMint in the balances of the Output with the hash of the OutputID. It returns a
// copy of the original Output with the modified balances.
func (o *ExtendedLockedOutput) UpdateMintingColor() Output {
	coloredBalances := o.Balances().Map()
	if mintedCoins, mintedCoinsExist := coloredBalances[ColorMint]; mintedCoinsExist {
		delete(coloredBalances, ColorMint)
		coloredBalances[Color(blake2b.Sum256(o.ID().Bytes()))] = mintedCoins
	}
	updatedOutput := NewExtendedLockedOutput(coloredBalances, o.Address())
	updatedOutput.SetID(o.ID())

	return updatedOutput
}

// Bytes returns a marshaled version of the Output.
func (o *ExtendedLockedOutput) Bytes() []byte {
	return o.ObjectStorageValue()
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (o *ExtendedLockedOutput) Update(objectstorage.StorableObject) {
	panic("ExtendedLockedOutput: updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (o *ExtendedLockedOutput) ObjectStorageKey() []byte {
	return o.id.Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (o *ExtendedLockedOutput) ObjectStorageValue() []byte {
	flags := o.compressFlags()
	ret := marshalutil.New().
		WriteByte(byte(ExtendedLockedOutputType)).
		WriteBytes(o.balances.Bytes()).
		WriteBytes(o.address.Bytes()).
		WriteByte(flags)
	if flagExtendedLockedOutputFallbackPresent&flags != 0 {
		ret.WriteBytes(o.fallbackAddress.Bytes()).
			WriteUint32(o.fallbackDeadline)
	}
	if flagExtendedLockedOutputTimeLockPresent&flags != 0 {
		ret.WriteUint32(o.timelock)
	}
	if flagExtendedLockedOutputPayloadPresent&flags != 0 {
		ret.WriteUint16(uint16(len(o.payload))).
			WriteBytes(o.payload)
	}
	return ret.Bytes()
}

// Compare offers a comparator for Outputs which returns -1 if the other Output is bigger, 1 if it is smaller and 0 if
// they are the same.
func (o *ExtendedLockedOutput) Compare(other Output) int {
	return bytes.Compare(o.Bytes(), other.Bytes())
}

// String returns a human readable version of the Output.
func (o *ExtendedLockedOutput) String() string {
	return stringify.Struct("ExtendedLockedOutput",
		stringify.StructField("id", o.ID()),
		stringify.StructField("address", o.address),
		stringify.StructField("balances", o.balances),
		stringify.StructField("fallbackAddress", o.fallbackAddress),
		stringify.StructField("fallbackDeadline", o.fallbackDeadline),
		stringify.StructField("timelock", o.timelock),
	)
}

func (o *ExtendedLockedOutput) GetPayload() []byte {
	return o.payload
}

// code contract (make sure the type implements all required methods)
var _ Output = &ExtendedLockedOutput{}

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
