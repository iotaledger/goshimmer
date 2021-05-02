package ledgerstate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/bitmask"
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

	// AliasOutputType represents an Output which makes a chain with optional governance
	AliasOutputType

	// ExtendedLockedOutputType represents an Output which extends SigLockedColoredOutput with alias locking and fallback
	ExtendedLockedOutputType
)

// String returns a human readable representation of the OutputType.
func (o OutputType) String() string {
	return [...]string{
		"SigLockedSingleOutputType",
		"SigLockedColoredOutputType",
		"AliasOutputType",
		"ExtendedLockedOutputType",
	}[o]
}

// OutputTypeFromString returns the output type from a string.
func OutputTypeFromString(ot string) (OutputType, error) {
	res, ok := map[string]OutputType{
		"SigLockedSingleOutputType":  SigLockedSingleOutputType,
		"SigLockedColoredOutputType": SigLockedColoredOutputType,
		"AliasOutputType":            AliasOutputType,
		"ExtendedLockedOutputType":   ExtendedLockedOutputType,
	}[ot]
	if !ok {
		return res, xerrors.New(fmt.Sprintf("unsupported output type: %s", ot))
	}
	return res, nil
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
	case AliasOutputType:
		if output, err = AliasOutputFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse AliasOutput: %w", err)
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

// Filter removes all elements from the Outputs that do not pass the given condition.
func (o Outputs) Filter(condition func(output Output) bool) (filteredOutputs Outputs) {
	filteredOutputs = make(Outputs, 0)
	for _, output := range o {
		if condition(output) {
			filteredOutputs = append(filteredOutputs, output)
		}
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

// Strings returns the Outputs in the form []transactionID:index.
func (o Outputs) Strings() (result []string) {
	for _, output := range o {
		result = append(result, fmt.Sprintf("%s:%d", output.ID().TransactionID().Base58(), output.ID().OutputIndex()))
	}

	return
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
	switch blk := unlockBlock.(type) {
	case *SignatureUnlockBlock:
		// unlocking by signature
		unlockValid = blk.AddressSignatureValid(s.address, tx.Essence().Bytes())

	case *AliasUnlockBlock:
		// unlocking by alias reference. The unlock is valid if:
		// - referenced alias output has same alias address
		// - it is not unlocked for governance
		if s.address.Type() != AliasAddressType {
			return false, xerrors.Errorf("SigLockedSingleOutput: %s address can't be unlocked by alias reference", s.address.Type().String())
		}
		refAliasOutput, isAlias := inputs[blk.AliasInputIndex()].(*AliasOutput)
		if !isAlias {
			return false, xerrors.New("SigLockedSingleOutput: referenced input must be AliasOutput")
		}
		if !s.address.Equals(refAliasOutput.GetAliasAddress()) {
			return false, xerrors.New("SigLockedSingleOutput: wrong alias referenced")
		}
		unlockValid = !refAliasOutput.hasToBeUnlockedForGovernanceUpdate(tx)

	default:
		err = xerrors.Errorf("SigLockedSingleOutput: unsupported unlock block type: %w", cerrors.ErrParseBytesFailed)
	}

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

// UpdateMintingColor does nothing for SigLockedSingleOutput
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
	switch blk := unlockBlock.(type) {
	case *SignatureUnlockBlock:
		// unlocking by signature
		unlockValid = blk.AddressSignatureValid(s.address, tx.Essence().Bytes())

	case *AliasUnlockBlock:
		// unlocking by alias reference. The unlock is valid if:
		// - referenced alias output has same alias address
		// - it is not unlocked for governance
		if s.address.Type() != AliasAddressType {
			return false, xerrors.Errorf("SigLockedColoredOutput: %s address can't be unlocked by alias reference", s.address.Type().String())
		}
		refAliasOutput, isAlias := inputs[blk.AliasInputIndex()].(*AliasOutput)
		if !isAlias {
			return false, xerrors.New("SigLockedColoredOutput: referenced input must be AliasOutput")
		}
		if !s.address.Equals(refAliasOutput.GetAliasAddress()) {
			return false, xerrors.New("SigLockedColoredOutput: wrong alias referenced")
		}
		unlockValid = !refAliasOutput.hasToBeUnlockedForGovernanceUpdate(tx)

	default:
		err = xerrors.Errorf("SigLockedColoredOutput: unsupported unlock block type: %w", cerrors.ErrParseBytesFailed)
	}

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

// region AliasOutput ///////////////////////////////////////////////////////////////////////////////////////

// DustThresholdAliasOutputIOTA is minimum number of iotas enforced for the output to be correct
// TODO protocol-wide dust threshold configuration
const DustThresholdAliasOutputIOTA = uint64(100)

// MaxOutputPayloadSize size limit on the data payload in the output.
const MaxOutputPayloadSize = 4 * 1024

// flags use to compress serialized bytes
const (
	flagAliasOutputGovernanceUpdate = uint(iota)
	flagAliasOutputGovernanceSet
	flagAliasOutputStateDataPresent
	flagAliasOutputGovernanceMetadataPresent
	flagAliasOutputImmutableDataPresent
	flagAliasOutputIsOrigin
	flagAliasOutputDelegationConstraint
	flagAliasOutputDelegationTimelockPresent
)

// AliasOutput represents output which defines as AliasAddress.
// It can only be used in a chained manner
type AliasOutput struct {
	// common for all outputs
	outputID      OutputID
	outputIDMutex sync.RWMutex
	balances      *ColoredBalances

	// aliasAddress becomes immutable after created for a lifetime. It is returned as Address()
	aliasAddress AliasAddress
	// address which controls the state and state data
	// It can only be changed by governing entity, if set. Otherwise it is self governed.
	// Can also be an AliasAddress
	stateAddress Address
	// state index is enforced incremental counter of state updates. The constraint is:
	// - start at 0 when chain in minted
	// - increase by 1 with each new chained output with isGovernanceUpdate == false
	// - do not change with any new chained output with isGovernanceUpdate == true
	stateIndex uint32
	// optional state metadata. nil means absent
	stateData []byte
	// optional governance level metadata, that can only be changed with a governance update
	governanceMetadata []byte
	// optional immutable data. It is set when AliasOutput is minted and can't be changed since
	// Useful for NFTs
	immutableData []byte
	// if the AliasOutput is chained in the transaction, the flags states if it is updating state or governance data.
	// unlock validation of the corresponding input depends on it.
	isGovernanceUpdate bool
	// governance address if set. It can be any address, unlocked by signature of alias address. Nil means self governed
	governingAddress Address
	// true if it is the first output in the chain
	isOrigin bool
	// true if the output is subject to the "delegation" constraint: upon transition tokens cannot be changed
	isDelegated bool
	// delegation timelock (optional). Before the timelock, only state transition is permitted, after the timelock, only
	// governance transition
	delegationTimelock time.Time

	objectstorage.StorableObjectFlags
}

// NewAliasOutputMint creates new AliasOutput as minting output, i.e. the one which does not contain corresponding input.
func NewAliasOutputMint(balances map[Color]uint64, stateAddr Address, immutableData ...[]byte) (*AliasOutput, error) {
	if !IsAboveDustThreshold(balances) {
		return nil, xerrors.New("AliasOutput: colored balances are below dust threshold")
	}
	if stateAddr == nil {
		return nil, xerrors.New("AliasOutput: mandatory state address cannot be nil")
	}
	ret := &AliasOutput{
		balances:     NewColoredBalances(balances),
		stateAddress: stateAddr,
		isOrigin:     true,
	}
	if len(immutableData) > 0 {
		ret.immutableData = immutableData[0]
	}
	if err := ret.checkBasicValidity(); err != nil {
		return nil, err
	}
	return ret, nil
}

// NewAliasOutputNext creates new AliasOutput as state transition from the previous one
func (a *AliasOutput) NewAliasOutputNext(governanceUpdate ...bool) *AliasOutput {
	ret := a.clone()
	ret.isOrigin = false
	ret.isGovernanceUpdate = false
	if len(governanceUpdate) > 0 {
		ret.isGovernanceUpdate = governanceUpdate[0]
	}
	if !ret.isGovernanceUpdate {
		ret.stateIndex = a.stateIndex + 1
	}
	return ret
}

// WithDelegation returns the output as a delegateds alias output.
func (a *AliasOutput) WithDelegation() *AliasOutput {
	a.isDelegated = true
	return a
}

// WithDelegationAndTimelock returns the output as a delegated alias output and a set delegation timelock.
func (a *AliasOutput) WithDelegationAndTimelock(lockUntil time.Time) *AliasOutput {
	a.isDelegated = true
	a.delegationTimelock = lockUntil
	return a
}

// AliasOutputFromMarshalUtil unmarshals a AliasOutput using a MarshalUtil (for easier unmarshaling).
func AliasOutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (*AliasOutput, error) {
	var ret *AliasOutput
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, xerrors.Errorf("AliasOutput: failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if OutputType(outputType) != AliasOutputType {
		return nil, xerrors.Errorf("AliasOutput: invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
	}
	ret = &AliasOutput{}
	flagsByte, err1 := marshalUtil.ReadByte()
	if err1 != nil {
		return nil, xerrors.Errorf("AliasOutput: failed to parse AliasOutput flags (%v): %w", err1, cerrors.ErrParseBytesFailed)
	}
	flags := bitmask.BitMask(flagsByte)
	ret.isOrigin = flags.HasBit(flagAliasOutputIsOrigin)
	ret.isGovernanceUpdate = flags.HasBit(flagAliasOutputGovernanceUpdate)
	ret.isDelegated = flags.HasBit(flagAliasOutputDelegationConstraint)

	addr, err2 := AliasAddressFromMarshalUtil(marshalUtil)
	if err2 != nil {
		return nil, xerrors.Errorf("AliasOutput: failed to parse alias address (%v): %w", err2, cerrors.ErrParseBytesFailed)
	}
	ret.aliasAddress = *addr
	cb, err3 := ColoredBalancesFromMarshalUtil(marshalUtil)
	if err3 != nil {
		return nil, xerrors.Errorf("AliasOutput: failed to parse colored balances: %w", err3)
	}
	ret.balances = cb
	ret.stateAddress, err = AddressFromMarshalUtil(marshalUtil)
	if err != nil {
		return nil, xerrors.Errorf("AliasOutput: failed to parse state address (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	ret.stateIndex, err = marshalUtil.ReadUint32()
	if err != nil {
		return nil, xerrors.Errorf("AliasOutput: failed to parse state address (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if flags.HasBit(flagAliasOutputStateDataPresent) {
		size, err4 := marshalUtil.ReadUint16()
		if err4 != nil {
			return nil, xerrors.Errorf("AliasOutput: failed to parse state data size: %w", err4)
		}
		ret.stateData, err = marshalUtil.ReadBytes(int(size))
		if err != nil {
			return nil, xerrors.Errorf("AliasOutput: failed to parse state data: %w", err)
		}
	}
	if flags.HasBit(flagAliasOutputGovernanceMetadataPresent) {
		size, err5 := marshalUtil.ReadUint16()
		if err5 != nil {
			return nil, xerrors.Errorf("AliasOutput: failed to parse governance metadata size: %w", err5)
		}
		ret.governanceMetadata, err = marshalUtil.ReadBytes(int(size))
		if err != nil {
			return nil, xerrors.Errorf("AliasOutput: failed to parse governance metadata data: %w", err)
		}
	}
	if flags.HasBit(flagAliasOutputImmutableDataPresent) {
		size, err6 := marshalUtil.ReadUint16()
		if err6 != nil {
			return nil, xerrors.Errorf("AliasOutput: failed to parse immutable data size: %w", err6)
		}
		ret.immutableData, err = marshalUtil.ReadBytes(int(size))
		if err != nil {
			return nil, xerrors.Errorf("AliasOutput: failed to parse immutable data: %w", err)
		}
	}
	if flags.HasBit(flagAliasOutputGovernanceSet) {
		ret.governingAddress, err = AddressFromMarshalUtil(marshalUtil)
		if err != nil {
			return nil, xerrors.Errorf("AliasOutput: failed to parse governing address (%v): %w", err, cerrors.ErrParseBytesFailed)
		}
	}
	if flags.HasBit(flagAliasOutputDelegationTimelockPresent) {
		ret.delegationTimelock, err = marshalUtil.ReadTime()
		if err != nil {
			return nil, xerrors.Errorf("AliasOutput: failed to parse delegation timelock (%v): %w", err, cerrors.ErrParseBytesFailed)
		}
	}
	if err7 := ret.checkBasicValidity(); err7 != nil {
		return nil, err7
	}
	return ret, nil
}

// SetBalances sets colored balances of the output
func (a *AliasOutput) SetBalances(balances map[Color]uint64) error {
	if !IsAboveDustThreshold(balances) {
		return xerrors.New("AliasOutput: balances are less than dust threshold")
	}
	a.balances = NewColoredBalances(balances)
	return nil
}

// GetAliasAddress calculates new ID if it is a minting output. Otherwise it takes stored value
func (a *AliasOutput) GetAliasAddress() *AliasAddress {
	if a.aliasAddress.IsNil() {
		return NewAliasAddress(a.ID().Bytes())
	}
	return &a.aliasAddress
}

// SetAliasAddress sets the alias address of the alias output.
func (a *AliasOutput) SetAliasAddress(addr *AliasAddress) {
	a.aliasAddress = *addr
}

// IsOrigin returns true if it starts the chain
func (a *AliasOutput) IsOrigin() bool {
	return a.isOrigin
}

// SetIsOrigin sets the isOrigin field of the output.
func (a *AliasOutput) SetIsOrigin(isOrigin bool) {
	a.isOrigin = isOrigin
}

// IsDelegated returns true if the output is delegated.
func (a *AliasOutput) IsDelegated() bool {
	return a.isDelegated
}

// SetIsDelegated sets the isDelegated field of the output.
func (a *AliasOutput) SetIsDelegated(isDelegated bool) {
	a.isDelegated = isDelegated
}

// IsSelfGoverned returns if governing address is not set which means that stateAddress is same as governingAddress
func (a *AliasOutput) IsSelfGoverned() bool {
	return a.governingAddress == nil
}

// GetStateAddress return state controlling address
func (a *AliasOutput) GetStateAddress() Address {
	return a.stateAddress
}

// SetStateAddress sets the state controlling address
func (a *AliasOutput) SetStateAddress(addr Address) error {
	if addr == nil {
		return xerrors.New("AliasOutput: mandatory state address cannot be nil")
	}
	a.stateAddress = addr
	return nil
}

// SetGoverningAddress sets the governing address or nil for self-governing
func (a *AliasOutput) SetGoverningAddress(addr Address) {
	if addr == nil {
		a.governingAddress = nil
		return
	}
	// calling Array on nil panics
	if addr.Array() == a.stateAddress.Array() {
		addr = nil // self governing
	}
	a.governingAddress = addr
}

// GetGoverningAddress return governing address. If self-governed, it is the same as state controlling address
func (a *AliasOutput) GetGoverningAddress() Address {
	if a.IsSelfGoverned() {
		return a.stateAddress
	}
	return a.governingAddress
}

// SetStateData sets state data
func (a *AliasOutput) SetStateData(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return xerrors.New("AliasOutput: state data too big")
	}
	a.stateData = make([]byte, len(data))
	copy(a.stateData, data)
	return nil
}

// GetStateData gets the state data
func (a *AliasOutput) GetStateData() []byte {
	return a.stateData
}

// SetGovernanceMetadata sets governance metadata
func (a *AliasOutput) SetGovernanceMetadata(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return xerrors.New("AliasOutput: governance metadata too big")
	}
	a.governanceMetadata = make([]byte, len(data))
	copy(a.governanceMetadata, data)
	return nil
}

// GetGovernanceMetadata gets the governance metadata
func (a *AliasOutput) GetGovernanceMetadata() []byte {
	return a.governanceMetadata
}

// SetStateIndex sets the state index in the input. It is enforced to increment by 1 with each state transition
func (a *AliasOutput) SetStateIndex(index uint32) {
	a.stateIndex = index
}

// GetIsGovernanceUpdated returns if the output was unlocked for governance in the transaction.
func (a *AliasOutput) GetIsGovernanceUpdated() bool {
	return a.isGovernanceUpdate
}

// SetIsGovernanceUpdated sets the isGovernanceUpdated flag.
func (a *AliasOutput) SetIsGovernanceUpdated(i bool) {
	a.isGovernanceUpdate = i
}

// GetStateIndex returns the state index
func (a *AliasOutput) GetStateIndex() uint32 {
	return a.stateIndex
}

// GetImmutableData gets the state data
func (a *AliasOutput) GetImmutableData() []byte {
	return a.immutableData
}

// SetImmutableData sets the immutable data field of the alias output.
func (a *AliasOutput) SetImmutableData(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return xerrors.New("AliasOutput: immutable data too big")
	}
	a.immutableData = make([]byte, len(data))
	copy(a.immutableData, data)
	return nil
}

// SetDelegationTimelock sets the delegation timelock. An error is returned if the output is not a delegated.
func (a *AliasOutput) SetDelegationTimelock(timelock time.Time) error {
	if !a.isDelegated {
		return xerrors.Errorf("AliasOutput: delegation timelock can only be set on a delegated output")
	}
	a.delegationTimelock = timelock
	return nil
}

// DelegationTimelock returns the delegation timelock. If the output is not delegated, or delegation timelock is
// not set, it returns the zero time object.
func (a *AliasOutput) DelegationTimelock() time.Time {
	if !a.isDelegated {
		return time.Time{}
	}
	return a.delegationTimelock
}

// DelegationTimeLockedNow determines if the alias output is delegation timelocked at a given time.
func (a *AliasOutput) DelegationTimeLockedNow(nowis time.Time) bool {
	if !a.isDelegated || a.delegationTimelock.IsZero() {
		return false
	}
	return a.delegationTimelock.After(nowis)
}

// Clone clones the structure
func (a *AliasOutput) Clone() Output {
	return a.clone()
}

func (a *AliasOutput) clone() *AliasOutput {
	a.mustValidate()
	ret := &AliasOutput{
		outputID:           a.outputID,
		balances:           a.balances.Clone(),
		aliasAddress:       *a.GetAliasAddress(),
		stateAddress:       a.stateAddress.Clone(),
		stateIndex:         a.stateIndex,
		stateData:          make([]byte, len(a.stateData)),
		governanceMetadata: make([]byte, len(a.governanceMetadata)),
		immutableData:      make([]byte, len(a.immutableData)),
		delegationTimelock: a.delegationTimelock,
		isOrigin:           a.isOrigin,
		isDelegated:        a.isDelegated,
		isGovernanceUpdate: a.isGovernanceUpdate,
	}
	if a.governingAddress != nil {
		ret.governingAddress = a.governingAddress.Clone()
	}
	copy(ret.stateData, a.stateData)
	copy(ret.governanceMetadata, a.governanceMetadata)
	copy(ret.immutableData, a.immutableData)
	ret.mustValidate()
	return ret
}

// ID is the ID of the output
func (a *AliasOutput) ID() OutputID {
	a.outputIDMutex.RLock()
	defer a.outputIDMutex.RUnlock()

	return a.outputID
}

// SetID set the output ID after unmarshalling
func (a *AliasOutput) SetID(outputID OutputID) Output {
	a.outputIDMutex.Lock()
	defer a.outputIDMutex.Unlock()

	a.outputID = outputID
	return a
}

// Type return the type of the output
func (a *AliasOutput) Type() OutputType {
	return AliasOutputType
}

// Balances return colored balances of the output
func (a *AliasOutput) Balances() *ColoredBalances {
	return a.balances
}

// Address AliasOutput is searchable in the ledger through its AliasAddress
func (a *AliasOutput) Address() Address {
	return a.GetAliasAddress()
}

// Input makes input from the output
func (a *AliasOutput) Input() Input {
	if a.ID() == EmptyOutputID {
		panic("AliasOutput: Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	}

	return NewUTXOInput(a.ID())
}

// Bytes serialized form
func (a *AliasOutput) Bytes() []byte {
	return a.ObjectStorageValue()
}

// String human readable form
func (a *AliasOutput) String() string {
	ret := "AliasOutput:\n"
	ret += fmt.Sprintf("   address: %s\n", a.Address())
	ret += fmt.Sprintf("   outputID: %s\n", a.ID())
	ret += fmt.Sprintf("   balance: %s\n", a.balances.String())
	ret += fmt.Sprintf("   stateAddress: %s\n", a.stateAddress)
	ret += fmt.Sprintf("   stateMetadataSize: %d\n", len(a.stateData))
	ret += fmt.Sprintf("   governingAddress (self-governed=%v): %s\n", a.IsSelfGoverned(), a.GetGoverningAddress())
	return ret
}

// Compare the two outputs
func (a *AliasOutput) Compare(other Output) int {
	return bytes.Compare(a.Bytes(), other.Bytes())
}

// Update is disabled
func (a *AliasOutput) Update(other objectstorage.StorableObject) {
	panic("AliasOutput: storage object updates disabled")
}

// ObjectStorageKey a key
func (a *AliasOutput) ObjectStorageKey() []byte {
	return a.ID().Bytes()
}

// ObjectStorageValue binary form
func (a *AliasOutput) ObjectStorageValue() []byte {
	flags := a.mustFlags()
	ret := marshalutil.New().
		WriteByte(byte(AliasOutputType)).
		WriteByte(byte(flags)).
		WriteBytes(a.aliasAddress.Bytes()).
		WriteBytes(a.balances.Bytes()).
		WriteBytes(a.stateAddress.Bytes()).
		WriteUint32(a.stateIndex)
	if flags.HasBit(flagAliasOutputStateDataPresent) {
		ret.WriteUint16(uint16(len(a.stateData))).
			WriteBytes(a.stateData)
	}
	if flags.HasBit(flagAliasOutputGovernanceMetadataPresent) {
		ret.WriteUint16(uint16(len(a.governanceMetadata))).
			WriteBytes(a.governanceMetadata)
	}
	if flags.HasBit(flagAliasOutputImmutableDataPresent) {
		ret.WriteUint16(uint16(len(a.immutableData))).
			WriteBytes(a.immutableData)
	}
	if flags.HasBit(flagAliasOutputGovernanceSet) {
		ret.WriteBytes(a.governingAddress.Bytes())
	}
	if flags.HasBit(flagAliasOutputDelegationTimelockPresent) {
		ret.WriteTime(a.delegationTimelock)
	}
	return ret.Bytes()
}

// UnlockValid check unlock and validates chain
func (a *AliasOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (bool, error) {
	// find the chained output in the tx
	chained, err := a.findChainedOutputAndCheckFork(tx)
	if err != nil {
		return false, err
	}
	switch blk := unlockBlock.(type) {
	case *SignatureUnlockBlock:
		// check signatures and validate transition
		if chained != nil {
			// chained output is present
			if chained.isGovernanceUpdate {
				// check if signature is valid against governing address
				if !blk.AddressSignatureValid(a.GetGoverningAddress(), tx.Essence().Bytes()) {
					return false, xerrors.New("signature is invalid for governance unlock")
				}
			} else {
				// check if signature is valid against state address
				if !blk.AddressSignatureValid(a.GetStateAddress(), tx.Essence().Bytes()) {
					return false, xerrors.New("signature is invalid for state unlock")
				}
			}
			// validate if transition passes the constraints
			if err := a.validateTransition(chained, tx); err != nil {
				return false, err
			}
		} else {
			// no chained output found. Alias is being destroyed?
			// check if governance is unlocked
			if !blk.AddressSignatureValid(a.GetGoverningAddress(), tx.Essence().Bytes()) {
				return false, xerrors.New("signature is invalid for chain output deletion")
			}
			// validate deletion constraint
			if err := a.validateDestroyTransitionNow(tx.Essence().Timestamp()); err != nil {
				return false, err
			}
		}
		return true, nil
	case *AliasUnlockBlock:
		// The referenced alias output should always be unlocked itself for state transition. But since the state address
		// can be an AliasAddress, the referenced alias may be unlocked by in turn an other referenced alias. This can cause
		// circular dependency among the unlock blocks, that results in all of them being unlocked without anyone having to
		// provide a signature. As a consequence, the circular dependencies of the alias unlock blocks is checked before
		// the UnlockValid() methods are called on any unlock blocks. We assume in this function that there is no such
		// circular dependency.
		if chained != nil {
			// chained output is present
			if chained.isGovernanceUpdate {
				if valid, err := a.unlockedGovernanceTransitionByAliasIndex(tx, blk.AliasInputIndex(), inputs); !valid {
					return false, xerrors.Errorf("referenced alias does not unlock alias for governance transition: %w", err)
				}
			} else {
				if valid, err := a.unlockedStateTransitionByAliasIndex(tx, blk.AliasInputIndex(), inputs); !valid {
					return false, xerrors.Errorf("referenced alias does not unlock alias for state transition: %w", err)
				}
			}
			// validate if transition passes the constraints
			if err := a.validateTransition(chained, tx); err != nil {
				return false, err
			}
		} else {
			// no chained output is present. Alias being destroyed?
			// check if alias is unlocked for governance transition by the referenced
			if valid, err := a.unlockedGovernanceTransitionByAliasIndex(tx, blk.AliasInputIndex(), inputs); !valid {
				return false, xerrors.Errorf("referenced alias does not unlock alias for governance transition: %w", err)
			}
			// validate deletion constraint
			if err := a.validateDestroyTransitionNow(tx.Essence().Timestamp()); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, xerrors.New("unsupported unlock block type")
}

// UpdateMintingColor replaces minting code with computed color code, and calculates the alias address if it is a
// freshly minted alias output
func (a *AliasOutput) UpdateMintingColor() Output {
	coloredBalances := a.Balances().Map()
	if mintedCoins, mintedCoinsExist := coloredBalances[ColorMint]; mintedCoinsExist {
		delete(coloredBalances, ColorMint)
		coloredBalances[Color(blake2b.Sum256(a.ID().Bytes()))] = mintedCoins
	}
	updatedOutput := a.clone()
	_ = updatedOutput.SetBalances(coloredBalances)
	updatedOutput.SetID(a.ID())

	if a.IsOrigin() {
		updatedOutput.SetAliasAddress(NewAliasAddress(a.ID().Bytes()))
	}

	return updatedOutput
}

// checkBasicValidity checks basic validity of the output
func (a *AliasOutput) checkBasicValidity() error {
	if !IsAboveDustThreshold(a.balances.Map()) {
		return xerrors.New("AliasOutput: balances are below dust threshold")
	}
	if a.stateAddress == nil {
		return xerrors.New("AliasOutput: state address must not be nil")
	}
	if a.IsOrigin() && a.stateIndex != 0 {
		return xerrors.New("AliasOutput: origin must have stateIndex == 0")
	}
	if len(a.stateData) > MaxOutputPayloadSize {
		return xerrors.Errorf("AliasOutput: size of the stateData (%d) exceeds maximum allowed (%d)",
			len(a.stateData), MaxOutputPayloadSize)
	}
	if len(a.governanceMetadata) > MaxOutputPayloadSize {
		return xerrors.Errorf("AliasOutput: size of the governance metadata (%d) exceeds maximum allowed (%d)",
			len(a.governanceMetadata), MaxOutputPayloadSize)
	}
	if len(a.immutableData) > MaxOutputPayloadSize {
		return xerrors.Errorf("AliasOutput: size of the immutableData (%d) exceeds maximum allowed (%d)",
			len(a.immutableData), MaxOutputPayloadSize)
	}
	if !a.isDelegated && !a.delegationTimelock.IsZero() {
		return xerrors.Errorf("AliasOutput: delegation timelock is present, but output is not delegated")
	}
	return nil
}

// mustValidate internal validity assertion
func (a *AliasOutput) mustValidate() {
	if err := a.checkBasicValidity(); err != nil {
		panic(err)
	}
}

// mustFlags produces flags for serialization
func (a *AliasOutput) mustFlags() bitmask.BitMask {
	a.mustValidate()
	var ret bitmask.BitMask
	if a.isOrigin {
		ret = ret.SetBit(flagAliasOutputIsOrigin)
	}
	if a.isGovernanceUpdate {
		ret = ret.SetBit(flagAliasOutputGovernanceUpdate)
	}
	if len(a.immutableData) > 0 {
		ret = ret.SetBit(flagAliasOutputImmutableDataPresent)
	}
	if len(a.stateData) > 0 {
		ret = ret.SetBit(flagAliasOutputStateDataPresent)
	}
	if a.governingAddress != nil {
		ret = ret.SetBit(flagAliasOutputGovernanceSet)
	}
	if len(a.governanceMetadata) > 0 {
		ret = ret.SetBit(flagAliasOutputGovernanceMetadataPresent)
	}
	if a.isDelegated {
		ret = ret.SetBit(flagAliasOutputDelegationConstraint)
	}
	if !a.delegationTimelock.IsZero() {
		ret = ret.SetBit(flagAliasOutputDelegationTimelockPresent)
	}
	return ret
}

// findChainedOutputAndCheckFork finds corresponding chained output.
// If it is not unique, returns an error
// If there's no such output, return nil and no error
func (a *AliasOutput) findChainedOutputAndCheckFork(tx *Transaction) (*AliasOutput, error) {
	var ret *AliasOutput
	aliasAddress := a.GetAliasAddress()
	for _, out := range tx.Essence().Outputs() {
		if out.Type() != AliasOutputType {
			continue
		}
		outAlias := out.(*AliasOutput)
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

// equalColoredBalances utility to compare colored balances
func equalColoredBalances(b1, b2 *ColoredBalances) bool {
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

// IsAboveDustThreshold internal utility to check if balances pass dust constraint
func IsAboveDustThreshold(m map[Color]uint64) bool {
	if iotas, ok := m[ColorIOTA]; ok && iotas >= DustThresholdAliasOutputIOTA {
		return true
	}
	return false
}

// IsExactDustMinimum checks if colored balances are exactly what is required by dust constraint
func IsExactDustMinimum(b *ColoredBalances) bool {
	bals := b.Map()
	if len(bals) != 1 {
		return false
	}
	bal, ok := bals[ColorIOTA]
	if !ok || bal != DustThresholdAliasOutputIOTA {
		return false
	}
	return true
}

// validateTransition enforces transition constraints between input and output chain outputs
func (a *AliasOutput) validateTransition(chained *AliasOutput, tx *Transaction) error {
	// enforce immutability of alias address and immutable data
	if !a.GetAliasAddress().Equals(chained.GetAliasAddress()) {
		return xerrors.New("AliasOutput: can't modify alias address")
	}
	if !bytes.Equal(a.immutableData, chained.immutableData) {
		return xerrors.New("AliasOutput: can't modify immutable data")
	}
	// depending on update type, enforce valid transition
	if chained.isGovernanceUpdate {
		// GOVERNANCE TRANSITION
		// should not modify state data
		if !bytes.Equal(a.stateData, chained.stateData) {
			return xerrors.New("AliasOutput: state data is not unlocked for modification")
		}
		// should not modify state index
		if a.stateIndex != chained.stateIndex {
			return xerrors.New("AliasOutput: state index is not unlocked for modification")
		}
		// should not modify tokens
		if !equalColoredBalances(a.balances, chained.balances) {
			return xerrors.New("AliasOutput: tokens are not unlocked for modification")
		}
		// if delegation timelock is set and active, governance transition is invalid
		// It means delegating party can't take funds back before timelock deadline
		if a.IsDelegated() && a.DelegationTimeLockedNow(tx.Essence().Timestamp()) {
			return xerrors.Errorf("AliasOutput: governance transition not allowed until %s, transaction timestamp is: %s",
				a.delegationTimelock.String(), tx.Essence().Timestamp().String())
		}
		// can modify state address
		// can modify governing address
		// can modify governance metadata
		// can modify delegation status
		// can modify delegation timelock
	} else {
		// STATE TRANSITION
		// can modify state data
		// should increment state index
		if a.stateIndex+1 != chained.stateIndex {
			return xerrors.Errorf("AliasOutput: expected state index is %d found %d", a.stateIndex+1, chained.stateIndex)
		}
		// can modify tokens
		// should not modify stateAddress
		if !a.stateAddress.Equals(chained.stateAddress) {
			return xerrors.New("AliasOutput: state address is not unlocked for modification")
		}
		// should not modify governing address
		if a.IsSelfGoverned() != chained.IsSelfGoverned() ||
			(a.governingAddress != nil && !a.governingAddress.Equals(chained.governingAddress)) {
			return xerrors.New("AliasOutput: governing address is not unlocked for modification")
		}
		// should not modify governance metadata
		if !bytes.Equal(a.governanceMetadata, chained.governanceMetadata) {
			return xerrors.New("AliasOutput: governance metadata is not unlocked for modification")
		}
		// should not modify token balances if delegation constraint is set
		if a.IsDelegated() && !equalColoredBalances(a.balances, chained.balances) {
			return xerrors.New("AliasOutput: delegated output funds can't be changed")
		}
		// should not modify delegation status in state transition
		if a.IsDelegated() != chained.IsDelegated() {
			return xerrors.New("AliasOutput: delegation status can't be changed")
		}
		// should not modify delegation timelock
		if !a.DelegationTimelock().Equal(chained.DelegationTimelock()) {
			return xerrors.New("AliasOutput: delegation timelock can't be changed")
		}
		// can only be accepted:
		//    - if no delegation timelock, state update can happen whenever
		//    - if delegation timelock is present, need to check if the timelock is active, otherwise state update not allowed
		if a.IsDelegated() && !a.DelegationTimelock().IsZero() && !a.DelegationTimeLockedNow(tx.Essence().Timestamp()) {
			return xerrors.Errorf("AliasOutput: state transition of delegated output not allowed after %s, transaction timestamp is %s",
				a.delegationTimelock.String(), tx.Essence().Timestamp().String())
		}
	}
	return nil
}

// validateDestroyTransitionNow check validity if input is not chained (destroyed)
func (a *AliasOutput) validateDestroyTransitionNow(nowis time.Time) error {
	if !IsExactDustMinimum(a.balances) {
		return xerrors.New("AliasOutput: didn't find chained output and there are more tokens then upper limit for alias destruction")
	}
	if a.IsDelegated() && a.DelegationTimeLockedNow(nowis) {
		return xerrors.New("AliasOutput: didn't find expected chained output for delegated output")
	}
	return nil
}

// unlockedGovernanceTransitionByAliasIndex unlocks one step of alias dereference for governance transition
func (a *AliasOutput) unlockedGovernanceTransitionByAliasIndex(tx *Transaction, refIndex uint16, inputs []Output) (bool, error) {
	// when output is self governed, a.GetGoverningAddress() returns the state address
	if a.GetGoverningAddress().Type() != AliasAddressType {
		return false, xerrors.New("AliasOutput: expected governing address of AliasAddress type")
	}
	if int(refIndex) > len(inputs) {
		return false, xerrors.New("AliasOutput: wrong alias reference index")
	}
	refInput, ok := inputs[refIndex].(*AliasOutput)
	if !ok {
		return false, xerrors.New("AliasOutput: the referenced output is not of AliasOutput type")
	}
	if !refInput.GetAliasAddress().Equals(a.GetGoverningAddress().(*AliasAddress)) {
		return false, xerrors.New("AliasOutput: wrong alias reference address")
	}
	// the referenced output must be unlocked for state update
	return !refInput.hasToBeUnlockedForGovernanceUpdate(tx), nil
}

// unlockedStateTransitionByAliasIndex unlocks one step of alias dereference for state transition
func (a *AliasOutput) unlockedStateTransitionByAliasIndex(tx *Transaction, refIndex uint16, inputs []Output) (bool, error) {
	// when output is self governed, a.GetGoverningAddress() returns the state address
	if a.GetStateAddress().Type() != AliasAddressType {
		return false, xerrors.New("AliasOutput: expected state address of AliasAddress type")
	}
	if int(refIndex) > len(inputs) {
		return false, xerrors.New("AliasOutput: wrong alias reference index")
	}
	refInput, ok := inputs[refIndex].(*AliasOutput)
	if !ok {
		return false, xerrors.New("AliasOutput: the referenced output is not of AliasOutput type")
	}
	if !refInput.GetAliasAddress().Equals(a.GetStateAddress().(*AliasAddress)) {
		return false, xerrors.New("AliasOutput: wrong alias reference address")
	}
	// the referenced output must be unlocked for state update
	return !refInput.hasToBeUnlockedForGovernanceUpdate(tx), nil
}

// hasToBeUnlockedForGovernanceUpdate finds chained output and checks if it is unlocked governance flags set
// If there's no chained output it means governance unlock is required to destroy the output
func (a *AliasOutput) hasToBeUnlockedForGovernanceUpdate(tx *Transaction) bool {
	chained, err := a.findChainedOutputAndCheckFork(tx)
	if err != nil {
		return false
	}
	if chained == nil {
		// the corresponding chained output not found, it means it is being destroyed,
		// for this we need governance unlock
		return true
	}
	return chained.isGovernanceUpdate
}

// code contract (make sure the type implements all required methods)
var _ Output = &AliasOutput{}

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
	fallbackAddress  Address
	fallbackDeadline time.Time

	// Deadline since when output can be unlocked
	timelock time.Time

	// any attached data (subject to size limits)
	payload []byte

	objectstorage.StorableObjectFlags
}

const (
	flagExtendedLockedOutputFallbackPresent = uint(iota)
	flagExtendedLockedOutputTimeLockPresent
	flagExtendedLockedOutputPayloadPresent
)

// NewExtendedLockedOutput is the constructor for a ExtendedLockedOutput.
func NewExtendedLockedOutput(balances map[Color]uint64, address Address) *ExtendedLockedOutput {
	return &ExtendedLockedOutput{
		balances: NewColoredBalances(balances),
		address:  address.Clone(),
	}
}

// WithFallbackOptions adds fallback options to the output and returns the updated version.
func (o *ExtendedLockedOutput) WithFallbackOptions(addr Address, deadline time.Time) *ExtendedLockedOutput {
	if addr != nil {
		o.fallbackAddress = addr.Clone()
	} else {
		o.fallbackAddress = nil
	}
	o.fallbackDeadline = deadline
	return o
}

// WithTimeLock adds timelock to the output and returns the updated version.
func (o *ExtendedLockedOutput) WithTimeLock(timelock time.Time) *ExtendedLockedOutput {
	o.timelock = timelock
	return o
}

// SetPayload sets the payload field of the output.
func (o *ExtendedLockedOutput) SetPayload(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return xerrors.Errorf("ExtendedLockedOutput: data payload size (%d bytes) is bigger than maximum allowed (%d bytes)", len(data), MaxOutputPayloadSize)
	}
	o.payload = make([]byte, len(data))
	copy(o.payload, data)
	return nil
}

// ExtendedOutputFromBytes unmarshals a ExtendedLockedOutput from a sequence of bytes.
func ExtendedOutputFromBytes(data []byte) (output *ExtendedLockedOutput, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(data)
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
	var flagsByte byte
	if flagsByte, err = marshalUtil.ReadByte(); err != nil {
		err = xerrors.Errorf("failed to parse flags (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	flags := bitmask.BitMask(flagsByte)
	if flags.HasBit(flagExtendedLockedOutputFallbackPresent) {
		if output.fallbackAddress, err = AddressFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse fallbackAddress (%v): %w", err, cerrors.ErrParseBytesFailed)
			return
		}
		if output.fallbackDeadline, err = marshalUtil.ReadTime(); err != nil {
			err = xerrors.Errorf("failed to parse fallbackTimeout (%v): %w", err, cerrors.ErrParseBytesFailed)
			return
		}
	}
	if flags.HasBit(flagExtendedLockedOutputTimeLockPresent) {
		if output.timelock, err = marshalUtil.ReadTime(); err != nil {
			err = xerrors.Errorf("failed to parse timelock (%v): %w", err, cerrors.ErrParseBytesFailed)
			return
		}
	}
	if flags.HasBit(flagExtendedLockedOutputPayloadPresent) {
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
	return output, nil
}

// compressFlags examines the optional fields of the output and returns the combined flags as a byte.
func (o *ExtendedLockedOutput) compressFlags() bitmask.BitMask {
	var ret bitmask.BitMask
	if o.fallbackAddress != nil {
		ret = ret.SetBit(flagExtendedLockedOutputFallbackPresent)
	}
	if !o.timelock.IsZero() {
		ret = ret.SetBit(flagExtendedLockedOutputTimeLockPresent)
	}
	if len(o.payload) > 0 {
		ret = ret.SetBit(flagExtendedLockedOutputPayloadPresent)
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
	if o.TimeLockedNow(tx.Essence().Timestamp()) {
		return false, nil
	}
	addr := o.UnlockAddressNow(tx.Essence().Timestamp())

	switch blk := unlockBlock.(type) {
	case *SignatureUnlockBlock:
		// unlocking by signature
		unlockValid = blk.AddressSignatureValid(addr, tx.Essence().Bytes())

	case *AliasUnlockBlock:
		// unlocking by alias reference. The unlock is valid if:
		// - referenced alias output has same alias address
		// - it is not unlocked for governance
		if addr.Type() != AliasAddressType {
			return false, xerrors.Errorf("ExtendedLockedOutput: %s address can't be unlocked by alias reference", addr.Type().String())
		}
		refAliasOutput, isAlias := inputs[blk.AliasInputIndex()].(*AliasOutput)
		if !isAlias {
			return false, xerrors.New("ExtendedLockedOutput: referenced input must be AliasOutput")
		}
		if !addr.Equals(refAliasOutput.GetAliasAddress()) {
			return false, xerrors.New("ExtendedLockedOutput: wrong alias referenced")
		}
		unlockValid = !refAliasOutput.hasToBeUnlockedForGovernanceUpdate(tx)

	default:
		err = xerrors.Errorf("ExtendedLockedOutput: unsupported unlock block type: %w", cerrors.ErrParseBytesFailed)
	}
	return
}

// Address returns the Address that the Output is associated to.
func (o *ExtendedLockedOutput) Address() Address {
	return o.address
}

// FallbackAddress returns the fallback address that the Output is associated to.
func (o *ExtendedLockedOutput) FallbackAddress() (addy Address) {
	if o.fallbackAddress == nil {
		return
	}
	return o.fallbackAddress
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
	ret := *o
	ret.balances = o.balances.Clone()
	ret.address = o.address.Clone()
	if o.fallbackAddress != nil {
		ret.fallbackAddress = o.fallbackAddress.Clone()
	}
	if o.payload != nil {
		ret.payload = make([]byte, len(o.payload))
		copy(ret.payload, o.payload)
	}
	return &ret
}

// UpdateMintingColor replaces the ColorMint in the balances of the Output with the hash of the OutputID. It returns a
// copy of the original Output with the modified balances.
func (o *ExtendedLockedOutput) UpdateMintingColor() Output {
	coloredBalances := o.Balances().Map()
	if mintedCoins, mintedCoinsExist := coloredBalances[ColorMint]; mintedCoinsExist {
		delete(coloredBalances, ColorMint)
		coloredBalances[Color(blake2b.Sum256(o.ID().Bytes()))] = mintedCoins
	}
	updatedOutput := NewExtendedLockedOutput(coloredBalances, o.Address()).
		WithFallbackOptions(o.fallbackAddress, o.fallbackDeadline).
		WithTimeLock(o.timelock)
	if err := updatedOutput.SetPayload(o.payload); err != nil {
		panic(xerrors.Errorf("UpdateMintingColor: %v", err))
	}
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
		WriteByte(byte(flags))
	if flags.HasBit(flagExtendedLockedOutputFallbackPresent) {
		ret.WriteBytes(o.fallbackAddress.Bytes()).
			WriteTime(o.fallbackDeadline)
	}
	if flags.HasBit(flagExtendedLockedOutputTimeLockPresent) {
		ret.WriteTime(o.timelock)
	}
	if flags.HasBit(flagExtendedLockedOutputPayloadPresent) {
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

// GetPayload return a data payload associated with the output
func (o *ExtendedLockedOutput) GetPayload() []byte {
	return o.payload
}

// TimeLock is a time after which output can be unlocked
func (o *ExtendedLockedOutput) TimeLock() time.Time {
	return o.timelock
}

// TimeLockedNow checks if output is unlocked for the specific moment
func (o *ExtendedLockedOutput) TimeLockedNow(nowis time.Time) bool {
	return o.TimeLock().After(nowis)
}

// FallbackOptions returns fallback options of the output. The address is nil if fallback options are not set
func (o *ExtendedLockedOutput) FallbackOptions() (Address, time.Time) {
	return o.fallbackAddress, o.fallbackDeadline
}

// UnlockAddressNow return unlock address which is valid for the specific moment of time
func (o *ExtendedLockedOutput) UnlockAddressNow(nowis time.Time) Address {
	if o.fallbackAddress == nil {
		return o.address
	}
	if nowis.After(o.fallbackDeadline) {
		return o.fallbackAddress
	}
	return o.address
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
func (c CachedOutputs) Unwrap() (unwrappedOutputs Outputs) {
	unwrappedOutputs = make(Outputs, len(c))
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
