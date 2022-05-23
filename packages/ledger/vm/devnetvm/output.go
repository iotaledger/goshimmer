package devnetvm

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/typeutils"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
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

// region Output ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Output is a generic interface for the different types of Outputs (with different unlock behaviors).
type Output interface {
	// ID returns the identifier of the Output that is used to address the Output in the UTXODAG.
	ID() utxo.OutputID

	// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
	// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
	// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
	// only accessed when the Output is supposed to be persisted.
	SetID(outputID utxo.OutputID)

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

	// String returns a human-readable version of the Output for debug purposes.
	String() string

	// Compare offers a comparator for Outputs which returns -1 if the other Output is bigger, 1 if it is smaller and 0
	// if they are the same.
	Compare(other Output) int

	// StorableObject makes Outputs storable in the ObjectStorage.
	objectstorage.StorableObject
}

// OutputFromBytes unmarshals an Output from a sequence of bytes.
func OutputFromBytes(bytes []byte) (outputEssence Output, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if outputEssence, err = OutputFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Output from MarshalUtil: %w", err)
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// OutputFromMarshalUtil unmarshals an Output using a MarshalUtil (for easier unmarshaling).
func OutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (outputEssence Output, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Errorf("failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	marshalUtil.ReadSeek(-1)

	switch OutputType(outputType) {
	case SigLockedSingleOutputType:
		if outputEssence, err = new(SigLockedSingleOutput).FromMarshalUtil(marshalUtil); err != nil {
			err = errors.Errorf("failed to parse SigLockedSingleOutput: %w", err)
			return
		}
	case SigLockedColoredOutputType:
		if outputEssence, err = new(SigLockedColoredOutput).FromMarshalUtil(marshalUtil); err != nil {
			err = errors.Errorf("failed to parse SigLockedColoredOutput: %w", err)
			return
		}
	case AliasOutputType:
		if outputEssence, err = new(AliasOutput).FromMarshalUtil(marshalUtil); err != nil {
			err = errors.Errorf("failed to parse AliasOutput: %w", err)
			return
		}
	case ExtendedLockedOutputType:
		if outputEssence, err = new(ExtendedLockedOutput).FromMarshalUtil(marshalUtil); err != nil {
			err = errors.Errorf("failed to parse ExtendedOutput: %w", err)
			return
		}

	default:
		err = errors.Errorf("unsupported OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// OutputFromObjectStorage restores an Output that was stored in the ObjectStorage.
func OutputFromObjectStorage(key []byte, data []byte) (output objectstorage.StorableObject, err error) {
	if output, _, err = OutputFromBytes(data); err != nil {
		return nil, errors.Errorf("failed to parse Output from bytes: %w", err)
	}

	var outputID utxo.OutputID
	if err = outputID.FromMarshalUtil(marshalutil.New(key)); err != nil {
		return nil, errors.Errorf("failed to parse OutputID from bytes: %w", err)
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

func OutputsFromUTXOOutputs(utxoOutputs utxo.Outputs) (outputs Outputs) {
	outputs = make(Outputs, 0)
	_ = utxoOutputs.ForEach(func(output utxo.Output) error {
		outputs = append(outputs, output.(Output))
		return nil
	})

	return outputs
}

// OutputsFromMarshalUtil unmarshals a collection of Outputs using a MarshalUtil (for easier unmarshalling).
func OutputsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (outputs Outputs, err error) {
	outputsCount, err := marshalUtil.ReadUint16()
	if err != nil {
		err = errors.Errorf("failed to parse outputs count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if outputsCount < MinOutputCount {
		err = errors.Errorf("amount of Outputs (%d) failed to reach MinOutputCount (%d): %w", outputsCount, MinOutputCount, cerrors.ErrParseBytesFailed)
		return
	}
	if outputsCount > MaxOutputCount {
		err = errors.Errorf("amount of Outputs (%d) exceeds MaxOutputCount (%d): %w", outputsCount, MaxOutputCount, cerrors.ErrParseBytesFailed)
		return
	}

	var previousOutput Output
	parsedOutputs := make([]Output, outputsCount)
	for i := uint16(0); i < outputsCount; i++ {
		if parsedOutputs[i], err = OutputFromMarshalUtil(marshalUtil); err != nil {
			return nil, errors.Errorf("failed to parse Output from MarshalUtil: %w", err)
		}

		if previousOutput != nil && previousOutput.Compare(parsedOutputs[i]) != -1 {
			return nil, errors.Errorf("order of Outputs is invalid: %w", cerrors.ErrParseBytesFailed)
		}
		previousOutput = parsedOutputs[i]
	}

	outputs = NewOutputs(parsedOutputs...)

	return
}

func (o Outputs) UTXOOutputs() (utxoOutputs []utxo.Output) {
	utxoOutputs = make([]utxo.Output, len(o))
	for i, output := range o {
		utxoOutputs[i] = output
	}

	return utxoOutputs
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

// String returns a human-readable version of the Outputs.
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
		result = append(result, fmt.Sprintf("%s", output.ID()))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsByID //////////////////////////////////////////////////////////////////////////////////////////////////

// OutputsByID represents a map of Outputs where every Output is stored with its corresponding OutputID as the key.
type OutputsByID map[utxo.OutputID]Output

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
	id      utxo.OutputID
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

// FromObjectStorage creates an SigLockedSingleOutput from sequences of key and bytes.
func (s *SigLockedSingleOutput) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	return s.FromBytes(byteutils.ConcatBytes(key, bytes))
}

// FromBytes unmarshals a SigLockedSingleOutput from a sequence of bytes.
func (s *SigLockedSingleOutput) FromBytes(bytes []byte) (output *SigLockedSingleOutput, err error) {
	marshalUtil := marshalutil.New(bytes)
	if output, err = s.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SigLockedSingleOutput from MarshalUtil: %w", err)
		return
	}
	return
}

// FromMarshalUtil unmarshals a SigLockedSingleOutput using a MarshalUtil (for easier unmarshaling).
func (s *SigLockedSingleOutput) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (output *SigLockedSingleOutput, err error) {
	if output = s; output == nil {
		output = new(SigLockedSingleOutput)
	}

	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Errorf("failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if OutputType(outputType) != SigLockedSingleOutputType {
		err = errors.Errorf("invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
		return
	}

	if output.balance, err = marshalUtil.ReadUint64(); err != nil {
		err = errors.Errorf("failed to parse balance (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if output.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Address (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	if output.balance < MinOutputBalance {
		err = errors.Errorf("balance (%d) is smaller than MinOutputBalance (%d): %w", output.balance, MinOutputBalance, cerrors.ErrParseBytesFailed)
		return
	}
	if output.balance > MaxOutputBalance {
		err = errors.Errorf("balance (%d) is bigger than MaxOutputBalance (%d): %w", output.balance, MaxOutputBalance, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// ID returns the identifier of the Output that is used to address the Output in the UTXODAG.
func (s *SigLockedSingleOutput) ID() utxo.OutputID {
	s.idMutex.RLock()
	defer s.idMutex.RUnlock()

	return s.id
}

// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (s *SigLockedSingleOutput) SetID(outputID utxo.OutputID) {
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
			return false, errors.Errorf("SigLockedSingleOutput: %s address can't be unlocked by alias reference", s.address.Type().String())
		}
		refAliasOutput, isAlias := inputs[blk.AliasInputIndex()].(*AliasOutput)
		if !isAlias {
			return false, errors.New("sigLockedSingleOutput: referenced input must be AliasOutput")
		}
		if !s.address.Equals(refAliasOutput.GetAliasAddress()) {
			return false, errors.New("sigLockedSingleOutput: wrong alias referenced")
		}
		unlockValid = !refAliasOutput.hasToBeUnlockedForGovernanceUpdate(tx)

	default:
		err = errors.Errorf("sigLockedSingleOutput: unsupported unlock block type: %w", cerrors.ErrParseBytesFailed)
	}

	return
}

// Address returns the Address that the Output is associated to.
func (s *SigLockedSingleOutput) Address() Address {
	return s.address
}

// Input returns an Input that references the Output.
func (s *SigLockedSingleOutput) Input() Input {
	if s.ID() == (utxo.OutputID{}) {
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

// UpdateMintingColor does nothing for SigLockedSingleOutput.
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
var _ Output = new(SigLockedSingleOutput)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SigLockedColoredOutput ///////////////////////////////////////////////////////////////////////////////////////

// SigLockedColoredOutput is an Output that holds colored balances and that can be unlocked by providing a signature for
// an Address.
type SigLockedColoredOutput struct {
	id       utxo.OutputID
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

// FromObjectStorage creates an SigLockedColoredOutput from sequences of key and bytes.
func (s *SigLockedColoredOutput) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	return s.FromBytes(byteutils.ConcatBytes(key, bytes))
}

// FromBytes unmarshals a SigLockedColoredOutput from a sequence of bytes.
func (s *SigLockedColoredOutput) FromBytes(bytes []byte) (output *SigLockedColoredOutput, err error) {
	marshalUtil := marshalutil.New(bytes)
	if output, err = s.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SigLockedColoredOutput from MarshalUtil: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals a SigLockedColoredOutput using a MarshalUtil (for easier unmarshaling).
func (s *SigLockedColoredOutput) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (output *SigLockedColoredOutput, err error) {
	if output = s; output == nil {
		output = new(SigLockedColoredOutput)
	}

	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Errorf("failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if OutputType(outputType) != SigLockedColoredOutputType {
		err = errors.Errorf("invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
		return
	}

	if output.balances, err = ColoredBalancesFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse ColoredBalances: %w", err)
		return
	}
	if output.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Address (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// ID returns the identifier of the Output that is used to address the Output in the UTXODAG.
func (s *SigLockedColoredOutput) ID() utxo.OutputID {
	s.idMutex.RLock()
	defer s.idMutex.RUnlock()

	return s.id
}

// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (s *SigLockedColoredOutput) SetID(outputID utxo.OutputID) {
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
			return false, errors.Errorf("SigLockedColoredOutput: %s address can't be unlocked by alias reference", s.address.Type().String())
		}
		refAliasOutput, isAlias := inputs[blk.AliasInputIndex()].(*AliasOutput)
		if !isAlias {
			return false, errors.New("sigLockedColoredOutput: referenced input must be AliasOutput")
		}
		if !s.address.Equals(refAliasOutput.GetAliasAddress()) {
			return false, errors.New("sigLockedColoredOutput: wrong alias referenced")
		}
		unlockValid = !refAliasOutput.hasToBeUnlockedForGovernanceUpdate(tx)

	default:
		err = errors.Errorf("sigLockedColoredOutput: unsupported unlock block type: %w", cerrors.ErrParseBytesFailed)
	}

	return
}

// Address returns the Address that the Output is associated to.
func (s *SigLockedColoredOutput) Address() Address {
	return s.address
}

// Input returns an Input that references the Output.
func (s *SigLockedColoredOutput) Input() Input {
	// TODO:
	// if s.ID() == (utxo.OutputID{}) {
	// 	panic("Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	// }

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
var _ Output = new(SigLockedColoredOutput)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AliasOutput ///////////////////////////////////////////////////////////////////////////////////////

// DustThresholdAliasOutputIOTA is minimum number of iotas enforced for the output to be correct
// TODO protocol-wide dust threshold configuration.
const DustThresholdAliasOutputIOTA = uint64(100)

// MaxOutputPayloadSize size limit on the data payload in the output.
const MaxOutputPayloadSize = 4 * 1024

// flags use to compress serialized bytes.
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
// It can only be used in a chained manner.
type AliasOutput struct {
	// common for all outputs
	outputID      utxo.OutputID
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
		return nil, errors.New("aliasOutput: colored balances are below dust threshold")
	}
	if stateAddr == nil {
		return nil, errors.New("aliasOutput: mandatory state address cannot be nil")
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

// NewAliasOutputNext creates new AliasOutput as state transition from the previous one.
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

// FromObjectStorage creates an AliasOutput from sequences of key and bytes.
func (a *AliasOutput) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	return a.FromBytes(byteutils.ConcatBytes(key, bytes))
}

// FromBytes unmarshals a ExtendedLockedOutput from a sequence of bytes.
func (a *AliasOutput) FromBytes(data []byte) (output *AliasOutput, err error) {
	marshalUtil := marshalutil.New(data)
	if output, err = a.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse AliasOutput from MarshalUtil: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals a AliasOutput using a MarshalUtil (for easier unmarshaling).
func (a *AliasOutput) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (output *AliasOutput, err error) {
	if output = a; output == nil {
		output = new(AliasOutput)
	}

	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, errors.Errorf("aliasOutput: failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if OutputType(outputType) != AliasOutputType {
		return nil, errors.Errorf("aliasOutput: invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
	}
	flagsByte, err1 := marshalUtil.ReadByte()
	if err1 != nil {
		return nil, errors.Errorf("aliasOutput: failed to parse AliasOutput flags (%v): %w", err1, cerrors.ErrParseBytesFailed)
	}
	flags := bitmask.BitMask(flagsByte)
	output.isOrigin = flags.HasBit(flagAliasOutputIsOrigin)
	output.isGovernanceUpdate = flags.HasBit(flagAliasOutputGovernanceUpdate)
	output.isDelegated = flags.HasBit(flagAliasOutputDelegationConstraint)

	addr, err2 := AliasAddressFromMarshalUtil(marshalUtil)
	if err2 != nil {
		return nil, errors.Errorf("aliasOutput: failed to parse alias address (%v): %w", err2, cerrors.ErrParseBytesFailed)
	}
	output.aliasAddress = *addr
	cb, err3 := ColoredBalancesFromMarshalUtil(marshalUtil)
	if err3 != nil {
		return nil, errors.Errorf("AliasOutput: failed to parse colored balances: %w", err3)
	}
	output.balances = cb
	output.stateAddress, err = AddressFromMarshalUtil(marshalUtil)
	if err != nil {
		return nil, errors.Errorf("aliasOutput: failed to parse state address (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	output.stateIndex, err = marshalUtil.ReadUint32()
	if err != nil {
		return nil, errors.Errorf("aliasOutput: failed to parse state address (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if flags.HasBit(flagAliasOutputStateDataPresent) {
		size, err4 := marshalUtil.ReadUint16()
		if err4 != nil {
			return nil, errors.Errorf("aliasOutput: failed to parse state data size: %w", err4)
		}
		output.stateData, err = marshalUtil.ReadBytes(int(size))
		if err != nil {
			return nil, errors.Errorf("aliasOutput: failed to parse state data: %w", err)
		}
	}
	if flags.HasBit(flagAliasOutputGovernanceMetadataPresent) {
		size, err5 := marshalUtil.ReadUint16()
		if err5 != nil {
			return nil, errors.Errorf("aliasOutput: failed to parse governance metadata size: %w", err5)
		}
		output.governanceMetadata, err = marshalUtil.ReadBytes(int(size))
		if err != nil {
			return nil, errors.Errorf("aliasOutput: failed to parse governance metadata data: %w", err)
		}
	}
	if flags.HasBit(flagAliasOutputImmutableDataPresent) {
		size, err6 := marshalUtil.ReadUint16()
		if err6 != nil {
			return nil, errors.Errorf("aliasOutput: failed to parse immutable data size: %w", err6)
		}
		output.immutableData, err = marshalUtil.ReadBytes(int(size))
		if err != nil {
			return nil, errors.Errorf("aliasOutput: failed to parse immutable data: %w", err)
		}
	}
	if flags.HasBit(flagAliasOutputGovernanceSet) {
		output.governingAddress, err = AddressFromMarshalUtil(marshalUtil)
		if err != nil {
			return nil, errors.Errorf("aliasOutput: failed to parse governing address (%v): %w", err, cerrors.ErrParseBytesFailed)
		}
	}
	if flags.HasBit(flagAliasOutputDelegationTimelockPresent) {
		output.delegationTimelock, err = marshalUtil.ReadTime()
		if err != nil {
			return nil, errors.Errorf("aliasOutput: failed to parse delegation timelock (%v): %w", err, cerrors.ErrParseBytesFailed)
		}
	}
	if err7 := output.checkBasicValidity(); err7 != nil {
		return nil, err7
	}
	return output, nil
}

// SetBalances sets colored balances of the output.
func (a *AliasOutput) SetBalances(balances map[Color]uint64) error {
	if !IsAboveDustThreshold(balances) {
		return errors.New("aliasOutput: balances are less than dust threshold")
	}
	a.balances = NewColoredBalances(balances)
	return nil
}

// GetAliasAddress calculates new ID if it is a minting output. Otherwise it takes stored value.
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

// IsOrigin returns true if it starts the chain.
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

// IsSelfGoverned returns if governing address is not set which means that stateAddress is same as governingAddress.
func (a *AliasOutput) IsSelfGoverned() bool {
	return a.governingAddress == nil
}

// GetStateAddress return state controlling address.
func (a *AliasOutput) GetStateAddress() Address {
	return a.stateAddress
}

// SetStateAddress sets the state controlling address.
func (a *AliasOutput) SetStateAddress(addr Address) error {
	if addr == nil {
		return errors.New("aliasOutput: mandatory state address cannot be nil")
	}
	a.stateAddress = addr
	return nil
}

// SetGoverningAddress sets the governing address or nil for self-governing.
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

// GetGoverningAddress return governing address. If self-governed, it is the same as state controlling address.
func (a *AliasOutput) GetGoverningAddress() Address {
	if a.IsSelfGoverned() {
		return a.stateAddress
	}
	return a.governingAddress
}

// SetStateData sets state data.
func (a *AliasOutput) SetStateData(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return errors.New("aliasOutput: state data too big")
	}
	a.stateData = make([]byte, len(data))
	copy(a.stateData, data)
	return nil
}

// GetStateData gets the state data.
func (a *AliasOutput) GetStateData() []byte {
	return a.stateData
}

// SetGovernanceMetadata sets governance metadata.
func (a *AliasOutput) SetGovernanceMetadata(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return errors.New("aliasOutput: governance metadata too big")
	}
	a.governanceMetadata = make([]byte, len(data))
	copy(a.governanceMetadata, data)
	return nil
}

// GetGovernanceMetadata gets the governance metadata.
func (a *AliasOutput) GetGovernanceMetadata() []byte {
	return a.governanceMetadata
}

// SetStateIndex sets the state index in the input. It is enforced to increment by 1 with each state transition.
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

// GetStateIndex returns the state index.
func (a *AliasOutput) GetStateIndex() uint32 {
	return a.stateIndex
}

// GetImmutableData gets the state data.
func (a *AliasOutput) GetImmutableData() []byte {
	return a.immutableData
}

// SetImmutableData sets the immutable data field of the alias output.
func (a *AliasOutput) SetImmutableData(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return errors.New("aliasOutput: immutable data too big")
	}
	a.immutableData = make([]byte, len(data))
	copy(a.immutableData, data)
	return nil
}

// SetDelegationTimelock sets the delegation timelock. An error is returned if the output is not a delegated.
func (a *AliasOutput) SetDelegationTimelock(timelock time.Time) error {
	if !a.isDelegated {
		return errors.Errorf("aliasOutput: delegation timelock can only be set on a delegated output")
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

// Clone clones the structure.
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

// ID is the ID of the output.
func (a *AliasOutput) ID() utxo.OutputID {
	a.outputIDMutex.RLock()
	defer a.outputIDMutex.RUnlock()

	return a.outputID
}

// SetID set the output ID after unmarshalling.
func (a *AliasOutput) SetID(outputID utxo.OutputID) {
	a.outputIDMutex.Lock()
	defer a.outputIDMutex.Unlock()

	a.outputID = outputID
}

// Type return the type of the output.
func (a *AliasOutput) Type() OutputType {
	return AliasOutputType
}

// Balances return colored balances of the output.
func (a *AliasOutput) Balances() *ColoredBalances {
	return a.balances
}

// Address AliasOutput is searchable in the ledger through its AliasAddress.
func (a *AliasOutput) Address() Address {
	return a.GetAliasAddress()
}

// Input makes input from the output.
func (a *AliasOutput) Input() Input {
	if a.ID() == (utxo.OutputID{}) {
		panic("AliasOutput: Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	}

	return NewUTXOInput(a.ID())
}

// Bytes serialized form.
func (a *AliasOutput) Bytes() []byte {
	return a.ObjectStorageValue()
}

// String human readable form.
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

// Compare the two outputs.
func (a *AliasOutput) Compare(other Output) int {
	return bytes.Compare(a.Bytes(), other.Bytes())
}

// ObjectStorageKey a key.
func (a *AliasOutput) ObjectStorageKey() []byte {
	return a.ID().Bytes()
}

// ObjectStorageValue binary form.
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

// UnlockValid check unlock and validates chain.
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
					return false, errors.New("signature is invalid for governance unlock")
				}
			} else {
				// check if signature is valid against state address
				if !blk.AddressSignatureValid(a.GetStateAddress(), tx.Essence().Bytes()) {
					return false, errors.New("signature is invalid for state unlock")
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
				return false, errors.New("signature is invalid for chain output deletion")
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
					return false, errors.Errorf("referenced alias does not unlock alias for governance transition: %w", err)
				}
			} else {
				if valid, err := a.unlockedStateTransitionByAliasIndex(tx, blk.AliasInputIndex(), inputs); !valid {
					return false, errors.Errorf("referenced alias does not unlock alias for state transition: %w", err)
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
				return false, errors.Errorf("referenced alias does not unlock alias for governance transition: %w", err)
			}
			// validate deletion constraint
			if err := a.validateDestroyTransitionNow(tx.Essence().Timestamp()); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, errors.New("unsupported unlock block type")
}

// UpdateMintingColor replaces minting code with computed color code, and calculates the alias address if it is a
// freshly minted alias output.
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

// checkBasicValidity checks basic validity of the output.
func (a *AliasOutput) checkBasicValidity() error {
	if !IsAboveDustThreshold(a.balances.Map()) {
		return errors.New("aliasOutput: balances are below dust threshold")
	}
	if a.stateAddress == nil {
		return errors.New("aliasOutput: state address must not be nil")
	}
	if a.IsOrigin() && a.stateIndex != 0 {
		return errors.New("aliasOutput: origin must have stateIndex == 0")
	}
	// a.aliasAddress is not set if the output is origin. It is only set after the output has been included in a tx, and
	// its outputID is known. To cover this edge case, TransactionFromMarshalUtil() performs the two checks below after
	// the ID has been set.
	if a.GetStateAddress().Equals(&a.aliasAddress) {
		return errors.New("state address cannot be the output's own alias address")
	}
	if a.GetGoverningAddress().Equals(&a.aliasAddress) {
		return errors.New("governing address cannot be the output's own alias address")
	}
	if len(a.stateData) > MaxOutputPayloadSize {
		return errors.Errorf("aliasOutput: size of the stateData (%d) exceeds maximum allowed (%d)",
			len(a.stateData), MaxOutputPayloadSize)
	}
	if len(a.governanceMetadata) > MaxOutputPayloadSize {
		return errors.Errorf("aliasOutput: size of the governance metadata (%d) exceeds maximum allowed (%d)",
			len(a.governanceMetadata), MaxOutputPayloadSize)
	}
	if len(a.immutableData) > MaxOutputPayloadSize {
		return errors.Errorf("aliasOutput: size of the immutableData (%d) exceeds maximum allowed (%d)",
			len(a.immutableData), MaxOutputPayloadSize)
	}
	if !a.isDelegated && !a.delegationTimelock.IsZero() {
		return errors.Errorf("aliasOutput: delegation timelock is present, but output is not delegated")
	}
	return nil
}

// mustValidate internal validity assertion.
func (a *AliasOutput) mustValidate() {
	if err := a.checkBasicValidity(); err != nil {
		panic(err)
	}
}

// mustFlags produces flags for serialization.
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
// If there's no such output, return nil and no error.
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
			return nil, errors.Errorf("duplicated alias output: %s", aliasAddress.String())
		}
		ret = outAlias
	}
	return ret, nil
}

// equalColoredBalances utility to compare colored balances.
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

// IsAboveDustThreshold internal utility to check if balances pass dust constraint.
func IsAboveDustThreshold(m map[Color]uint64) bool {
	if iotas, ok := m[ColorIOTA]; ok && iotas >= DustThresholdAliasOutputIOTA {
		return true
	}
	return false
}

// IsExactDustMinimum checks if colored balances are exactly what is required by dust constraint.
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

// validateTransition enforces transition constraints between input and output chain outputs.
func (a *AliasOutput) validateTransition(chained *AliasOutput, tx *Transaction) error {
	// enforce immutability of alias address and immutable data
	if !a.GetAliasAddress().Equals(chained.GetAliasAddress()) {
		return errors.New("aliasOutput: can't modify alias address")
	}
	if !bytes.Equal(a.immutableData, chained.immutableData) {
		return errors.New("aliasOutput: can't modify immutable data")
	}
	// depending on update type, enforce valid transition
	if chained.isGovernanceUpdate {
		// GOVERNANCE TRANSITION
		// should not modify state data
		if !bytes.Equal(a.stateData, chained.stateData) {
			return errors.New("aliasOutput: state data is not unlocked for modification")
		}
		// should not modify state index
		if a.stateIndex != chained.stateIndex {
			return errors.New("aliasOutput: state index is not unlocked for modification")
		}
		// should not modify tokens
		if !equalColoredBalances(a.balances, chained.balances) {
			return errors.New("aliasOutput: tokens are not unlocked for modification")
		}
		// if delegation timelock is set and active, governance transition is invalid
		// It means delegating party can't take funds back before timelock deadline
		if a.IsDelegated() && a.DelegationTimeLockedNow(tx.Essence().Timestamp()) {
			return errors.Errorf("aliasOutput: governance transition not allowed until %s, transaction timestamp is: %s",
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
			return errors.Errorf("aliasOutput: expected state index is %d found %d", a.stateIndex+1, chained.stateIndex)
		}
		// can modify tokens
		// should not modify stateAddress
		if !a.stateAddress.Equals(chained.stateAddress) {
			return errors.New("aliasOutput: state address is not unlocked for modification")
		}
		// should not modify governing address
		if a.IsSelfGoverned() != chained.IsSelfGoverned() ||
			(a.governingAddress != nil && !a.governingAddress.Equals(chained.governingAddress)) {
			return errors.New("aliasOutput: governing address is not unlocked for modification")
		}
		// should not modify governance metadata
		if !bytes.Equal(a.governanceMetadata, chained.governanceMetadata) {
			return errors.New("aliasOutput: governance metadata is not unlocked for modification")
		}
		// should not modify token balances if delegation constraint is set
		if a.IsDelegated() && !equalColoredBalances(a.balances, chained.balances) {
			return errors.New("aliasOutput: delegated output funds can't be changed")
		}
		// should not modify delegation status in state transition
		if a.IsDelegated() != chained.IsDelegated() {
			return errors.New("aliasOutput: delegation status can't be changed")
		}
		// should not modify delegation timelock
		if !a.DelegationTimelock().Equal(chained.DelegationTimelock()) {
			return errors.New("aliasOutput: delegation timelock can't be changed")
		}
		// can only be accepted:
		//    - if no delegation timelock, state update can happen whenever
		//    - if delegation timelock is present, need to check if the timelock is active, otherwise state update not allowed
		if a.IsDelegated() && !a.DelegationTimelock().IsZero() && !a.DelegationTimeLockedNow(tx.Essence().Timestamp()) {
			return errors.Errorf("aliasOutput: state transition of delegated output not allowed after %s, transaction timestamp is %s",
				a.delegationTimelock.String(), tx.Essence().Timestamp().String())
		}
	}
	return nil
}

// validateDestroyTransitionNow check validity if input is not chained (destroyed).
func (a *AliasOutput) validateDestroyTransitionNow(nowis time.Time) error {
	if !a.IsDelegated() && !IsExactDustMinimum(a.balances) {
		// if the output is delegated, it can be destroyed with more than minimum balance
		return errors.New("aliasOutput: didn't find chained output and there are more tokens then upper limit for non-delegated alias destruction")
	}
	if a.IsDelegated() && a.DelegationTimeLockedNow(nowis) {
		return errors.New("aliasOutput: didn't find expected chained output for delegated output")
	}
	return nil
}

// unlockedGovernanceTransitionByAliasIndex unlocks one step of alias dereference for governance transition.
func (a *AliasOutput) unlockedGovernanceTransitionByAliasIndex(tx *Transaction, refIndex uint16, inputs []Output) (bool, error) {
	// when output is self governed, a.GetGoverningAddress() returns the state address
	if a.GetGoverningAddress().Type() != AliasAddressType {
		return false, errors.New("aliasOutput: expected governing address of AliasAddress type")
	}
	if int(refIndex) > len(inputs) {
		return false, errors.New("aliasOutput: wrong alias reference index")
	}
	refInput, ok := inputs[refIndex].(*AliasOutput)
	if !ok {
		return false, errors.New("aliasOutput: the referenced output is not of AliasOutput type")
	}
	if !refInput.GetAliasAddress().Equals(a.GetGoverningAddress().(*AliasAddress)) {
		return false, errors.New("aliasOutput: wrong alias reference address")
	}
	// the referenced output must be unlocked for state update
	return !refInput.hasToBeUnlockedForGovernanceUpdate(tx), nil
}

// unlockedStateTransitionByAliasIndex unlocks one step of alias dereference for state transition.
func (a *AliasOutput) unlockedStateTransitionByAliasIndex(tx *Transaction, refIndex uint16, inputs []Output) (bool, error) {
	// when output is self governed, a.GetGoverningAddress() returns the state address
	if a.GetStateAddress().Type() != AliasAddressType {
		return false, errors.New("aliasOutput: expected state address of AliasAddress type")
	}
	if int(refIndex) > len(inputs) {
		return false, errors.New("aliasOutput: wrong alias reference index")
	}
	refInput, ok := inputs[refIndex].(*AliasOutput)
	if !ok {
		return false, errors.New("aliasOutput: the referenced output is not of AliasOutput type")
	}
	if !refInput.GetAliasAddress().Equals(a.GetStateAddress().(*AliasAddress)) {
		return false, errors.New("aliasOutput: wrong alias reference address")
	}
	// the referenced output must be unlocked for state update
	return !refInput.hasToBeUnlockedForGovernanceUpdate(tx), nil
}

// hasToBeUnlockedForGovernanceUpdate finds chained output and checks if it is unlocked governance flags set
// If there's no chained output it means governance unlock is required to destroy the output.
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

// code contract (make sure the type implements all required methods).
var _ Output = new(AliasOutput)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ExtendedLockedOutput /////////////////////////////////////////////////////////////////////////////////////////////////

// ExtendedLockedOutput is an Extension of SigLockedColoredOutput. If extended options not enabled,
// it behaves as SigLockedColoredOutput.
// In addition it has options:
// - fallback address and fallback timeout
// - can be unlocked by AliasUnlockBlock (if address is of AliasAddress type)
// - can be time locked until deadline
// - data payload for arbitrary metadata (size limits apply).
type ExtendedLockedOutput struct {
	id       utxo.OutputID
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
		return errors.Errorf("ExtendedLockedOutput: data payload size (%d bytes) is bigger than maximum allowed (%d bytes)", len(data), MaxOutputPayloadSize)
	}
	o.payload = make([]byte, len(data))
	copy(o.payload, data)
	return nil
}

// FromObjectStorage creates an ExtendedLockedOutput from sequences of key and bytes.
func (o *ExtendedLockedOutput) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	return o.FromBytes(byteutils.ConcatBytes(key, bytes))
}

// FromBytes unmarshals a ExtendedLockedOutput from a sequence of bytes.
func (o *ExtendedLockedOutput) FromBytes(data []byte) (output *ExtendedLockedOutput, err error) {
	marshalUtil := marshalutil.New(data)
	if output, err = o.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse ExtendedLockedOutput from MarshalUtil: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals a ExtendedLockedOutput using a MarshalUtil (for easier unmarshalling).
func (o *ExtendedLockedOutput) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (output *ExtendedLockedOutput, err error) {
	if output = o; output == nil {
		output = new(ExtendedLockedOutput)
	}

	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Errorf("failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if OutputType(outputType) != ExtendedLockedOutputType {
		err = errors.Errorf("invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
		return
	}

	if output.balances, err = ColoredBalancesFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse ColoredBalances: %w", err)
		return
	}
	if output.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Address (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	var flagsByte byte
	if flagsByte, err = marshalUtil.ReadByte(); err != nil {
		err = errors.Errorf("failed to parse flags (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	flags := bitmask.BitMask(flagsByte)
	if flags.HasBit(flagExtendedLockedOutputFallbackPresent) {
		if output.fallbackAddress, err = AddressFromMarshalUtil(marshalUtil); err != nil {
			err = errors.Errorf("failed to parse fallbackAddress (%v): %w", err, cerrors.ErrParseBytesFailed)
			return
		}
		if output.fallbackDeadline, err = marshalUtil.ReadTime(); err != nil {
			err = errors.Errorf("failed to parse fallbackTimeout (%v): %w", err, cerrors.ErrParseBytesFailed)
			return
		}
	}
	if flags.HasBit(flagExtendedLockedOutputTimeLockPresent) {
		if output.timelock, err = marshalUtil.ReadTime(); err != nil {
			err = errors.Errorf("failed to parse timelock (%v): %w", err, cerrors.ErrParseBytesFailed)
			return
		}
	}
	if flags.HasBit(flagExtendedLockedOutputPayloadPresent) {
		var size uint16
		size, err = marshalUtil.ReadUint16()
		if err != nil {
			err = errors.Errorf("failed to parse payload size (%v): %w", err, cerrors.ErrParseBytesFailed)
			return
		}
		output.payload, err = marshalUtil.ReadBytes(int(size))
		if err != nil {
			err = errors.Errorf("failed to parse payload (%v): %w", err, cerrors.ErrParseBytesFailed)
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
func (o *ExtendedLockedOutput) ID() utxo.OutputID {
	o.idMutex.RLock()
	defer o.idMutex.RUnlock()

	return o.id
}

// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (o *ExtendedLockedOutput) SetID(outputID utxo.OutputID) {
	o.idMutex.Lock()
	defer o.idMutex.Unlock()

	o.id = outputID
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
			return false, errors.Errorf("extendedLockedOutput: %s address can't be unlocked by alias reference", addr.Type().String())
		}
		refAliasOutput, isAlias := inputs[blk.AliasInputIndex()].(*AliasOutput)
		if !isAlias {
			return false, errors.New("extendedLockedOutput: referenced input must be AliasOutput")
		}
		if !addr.Equals(refAliasOutput.GetAliasAddress()) {
			return false, errors.New("extendedLockedOutput: wrong alias referenced")
		}
		unlockValid = !refAliasOutput.hasToBeUnlockedForGovernanceUpdate(tx)

	default:
		err = errors.Errorf("extendedLockedOutput: unsupported unlock block type: %w", cerrors.ErrParseBytesFailed)
	}
	return unlockValid, err
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
	if o.ID() == (utxo.OutputID{}) {
		panic("ExtendedLockedOutput: Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	}

	return NewUTXOInput(o.ID())
}

// Clone creates a copy of the Output.
func (o *ExtendedLockedOutput) Clone() Output {
	ret := &ExtendedLockedOutput{
		balances: o.balances.Clone(),
		address:  o.address.Clone(),
	}
	ret.id = o.id
	if o.fallbackAddress != nil {
		ret.fallbackAddress = o.fallbackAddress.Clone()
	}
	if !o.fallbackDeadline.IsZero() {
		ret.fallbackDeadline = o.fallbackDeadline
	}
	if !o.timelock.IsZero() {
		ret.timelock = o.timelock
	}
	if o.payload != nil {
		ret.payload = make([]byte, len(o.payload))
		copy(ret.payload, o.payload)
	}
	return ret
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
		panic(errors.Errorf("UpdateMintingColor: %v", err))
	}
	updatedOutput.SetID(o.ID())

	return updatedOutput
}

// Bytes returns a marshaled version of the Output.
func (o *ExtendedLockedOutput) Bytes() []byte {
	return o.ObjectStorageValue()
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

// GetPayload return a data payload associated with the output.
func (o *ExtendedLockedOutput) GetPayload() []byte {
	return o.payload
}

// TimeLock is a time after which output can be unlocked.
func (o *ExtendedLockedOutput) TimeLock() time.Time {
	return o.timelock
}

// TimeLockedNow checks if output is unlocked for the specific moment.
func (o *ExtendedLockedOutput) TimeLockedNow(nowis time.Time) bool {
	return o.TimeLock().After(nowis)
}

// FallbackOptions returns fallback options of the output. The address is nil if fallback options are not set.
func (o *ExtendedLockedOutput) FallbackOptions() (Address, time.Time) {
	return o.fallbackAddress, o.fallbackDeadline
}

// UnlockAddressNow return unlock address which is valid for the specific moment of time.
func (o *ExtendedLockedOutput) UnlockAddressNow(nowis time.Time) Address {
	if o.fallbackAddress == nil {
		return o.address
	}
	if nowis.After(o.fallbackDeadline) {
		return o.fallbackAddress
	}
	return o.address
}

// code contract (make sure the type implements all required methods).
var _ Output = new(ExtendedLockedOutput)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
