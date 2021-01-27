package ledgerstate

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/typeutils"
	"golang.org/x/xerrors"
)

// region InputType ////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// UTXOInputType is the type of an Input that references an UTXO Output.
	UTXOInputType InputType = iota
)

// InputType represents the type of an Input.
type InputType uint8

// InputTypeNames contains the names of the existing InputTypes.
var InputTypeNames = [...]string{
	"UTXOInputType",
}

// String returns a human readable representation of the InputType.
func (i InputType) String() string {
	if i > InputType(len(InputTypeNames)-1) {
		return fmt.Sprintf("InputType(%X)", uint8(i))
	}

	return InputTypeNames[i]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Input ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Input is a generic interface for different kinds of Inputs.
type Input interface {
	// Type returns the type of the Input.
	Type() InputType

	// Bytes returns a marshaled version of the Input.
	Bytes() []byte

	// String returns a human readable version of the Input.
	String() string

	// Compare offers a comparator for Inputs which returns -1 if other Input is bigger, 1 if it is smaller and 0 if they
	// are the same.
	Compare(other Input) int
}

// InputFromBytes unmarshals an Input from a sequence of bytes.
func InputFromBytes(inputBytes []byte) (input Input, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(inputBytes)
	if input, err = InputFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Input from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// InputFromMarshalUtil unmarshals an Input using a MarshalUtil (for easier unmarshaling).
func InputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (input Input, err error) {
	inputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse InputType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	marshalUtil.ReadSeek(-1)

	switch InputType(inputType) {
	case UTXOInputType:
		if input, err = UTXOInputFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse UTXOInput from MarshalUtil: %w", err)
			return
		}
	default:
		err = xerrors.Errorf("unsupported InputType (%X): %w", inputType, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Inputs ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Inputs represents a collection of Inputs that ensures a deterministic order.
type Inputs []Input

// NewInputs returns a deterministically ordered collection of Inputs removing existing duplicates.
func NewInputs(optionalInputs ...Input) (inputs Inputs) {
	seenInputs := make(map[string]types.Empty)
	sortedInputs := make([]struct {
		input           Input
		inputSerialized []byte
	}, 0)

	// filter duplicates (store marshaled version so we don't need to marshal a second time during sort)
	for _, input := range optionalInputs {
		marshaledInput := input.Bytes()
		marshaledInputAsString := typeutils.BytesToString(marshaledInput)

		if _, seenAlready := seenInputs[marshaledInputAsString]; seenAlready {
			continue
		}
		seenInputs[marshaledInputAsString] = types.Void

		sortedInputs = append(sortedInputs, struct {
			input           Input
			inputSerialized []byte
		}{input, marshaledInput})
	}

	// sort inputs
	sort.Slice(sortedInputs, func(i, j int) bool {
		return bytes.Compare(sortedInputs[i].inputSerialized, sortedInputs[j].inputSerialized) < 0
	})

	// create result
	inputs = make(Inputs, len(sortedInputs))
	for i, sortedInput := range sortedInputs {
		inputs[i] = sortedInput.input
	}

	return
}

// InputsFromBytes unmarshals a collection of Inputs from a sequence of bytes.
func InputsFromBytes(inputBytes []byte) (inputs Inputs, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(inputBytes)
	if inputs, err = InputsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Inputs from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// InputsFromMarshalUtil unmarshals a collection of Inputs using a MarshalUtil (for easier unmarshaling).
func InputsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (inputs Inputs, err error) {
	inputsCount, err := marshalUtil.ReadUint16()
	if err != nil {
		err = xerrors.Errorf("failed to parse inputs count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	var previousInput Input
	parsedInputs := make([]Input, inputsCount)
	for i := uint16(0); i < inputsCount; i++ {
		if parsedInputs[i], err = InputFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse Input from MarshalUtil: %w", err)
			return
		}

		if previousInput != nil && previousInput.Compare(parsedInputs[i]) != -1 {
			err = xerrors.Errorf("order of Inputs is invalid: %w", ErrTransactionInvalid)
			return
		}
		previousInput = parsedInputs[i]
	}

	inputs = NewInputs(parsedInputs...)

	return
}

// Clone creates a copy of the Inputs.
func (i Inputs) Clone() (clonedInputs Inputs) {
	clonedInputs = make(Inputs, len(i))
	copy(clonedInputs[:], i)

	return
}

// Bytes returns a marshaled version of the Inputs.
func (i Inputs) Bytes() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint16(uint16(len(i)))
	for _, input := range i {
		marshalUtil.WriteBytes(input.Bytes())
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the Inputs.
func (i Inputs) String() string {
	structBuilder := stringify.StructBuilder("Inputs")
	for i, input := range i {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), input))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UTXOInput ////////////////////////////////////////////////////////////////////////////////////////////////////

// UTXOInput represents a reference to an Output in the UTXODAG.
type UTXOInput struct {
	referencedOutputID OutputID
}

// NewUTXOInput is the constructor for UTXOInputs.
func NewUTXOInput(referencedOutputID OutputID) *UTXOInput {
	return &UTXOInput{
		referencedOutputID: referencedOutputID,
	}
}

// UTXOInputFromMarshalUtil unmarshals a UTXOInput using a MarshalUtil (for easier unmarshaling).
func UTXOInputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (input *UTXOInput, err error) {
	inputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse InputType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if InputType(inputType) != UTXOInputType {
		err = xerrors.Errorf("invalid InputType (%X): %w", inputType, cerrors.ErrParseBytesFailed)
		return
	}

	input = &UTXOInput{}
	if input.referencedOutputID, err = OutputIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse referenced OutputID from MarshalUtil: %w", err)
		return
	}

	return
}

// Type returns the type of the Input.
func (u *UTXOInput) Type() InputType {
	return UTXOInputType
}

// ReferencedOutputID returns the OutputID that this Input references.
func (u *UTXOInput) ReferencedOutputID() OutputID {
	return u.referencedOutputID
}

// Bytes returns a marshaled version of the Input.
func (u *UTXOInput) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(UTXOInputType)}, u.referencedOutputID.Bytes())
}

// Compare offers a comparator for Inputs which returns -1 if other Input is bigger, 1 if it is smaller and 0 if they
// are the same.
func (u *UTXOInput) Compare(other Input) int {
	return bytes.Compare(u.Bytes(), other.Bytes())
}

// String returns a human readable version of the Input.
func (u *UTXOInput) String() string {
	return stringify.Struct("UTXOInput",
		stringify.StructField("referencedOutputID", u.referencedOutputID),
	)
}

// code contract (make sure the struct implements all required methods)
var _ Input = &UTXOInput{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
