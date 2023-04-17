package utxo

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/orderedmap"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/serializer/v2/byteutils"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
	"github.com/iotaledger/hive.go/stringify"
)

// region TransactionID ////////////////////////////////////////////////////////////////////////////////////////////////

// TransactionID is a unique identifier for a Transaction.
type TransactionID struct {
	types.Identifier `serix:"0"`
}

// NewTransactionID returns a new TransactionID for the given data.
func NewTransactionID(txData []byte) (newTransactionID TransactionID) {
	return TransactionID{
		types.NewIdentifier(txData),
	}
}

// Bytes returns a serialized version of the TransactionID.
func (t TransactionID) Bytes() (serialized []byte, err error) {
	return t.Identifier.Bytes(), nil
}

// FromBytes un-serializes a TransactionID from a []byte.
func (t *TransactionID) FromBytes(data []byte) (consumedBytes int, err error) {
	return t.Identifier.Decode(data)
}

// Length returns the byte length of a serialized TransactionID.
func (t TransactionID) Length() (length int) {
	return types.IdentifierLength
}

// IsEmpty returns true if the TransactionID is empty.
func (t TransactionID) IsEmpty() (isEmpty bool) {
	return t == EmptyTransactionID
}

// String returns a human-readable version of the TransactionID.
func (t TransactionID) String() (humanReadable string) {
	return "TransactionID(" + t.Alias() + ")"
}

// EmptyTransactionID contains the null-value of the TransactionID type.
var EmptyTransactionID TransactionID

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionIDs ///////////////////////////////////////////////////////////////////////////////////////////////

// TransactionIDs represents a collection of TransactionIDs.
type TransactionIDs = *advancedset.AdvancedSet[TransactionID]

// NewTransactionIDs returns a new TransactionID collection with the given elements.
func NewTransactionIDs(ids ...TransactionID) (newTransactionIDs TransactionIDs) {
	return advancedset.New(ids...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

// OutputID is a unique identifier for an Output.
type OutputID struct {
	TransactionID TransactionID `serix:"0"`
	Index         uint16        `serix:"1"`
}

// NewOutputID returns a new OutputID for the given details.
func NewOutputID(txID TransactionID, index uint16) OutputID {
	return OutputID{
		TransactionID: txID,
		Index:         index,
	}
}

// FromBase58 un-serializes an OutputID from a base58 encoded string.
func (o *OutputID) FromBase58(base58EncodedString string) (err error) {
	decodedBytes, err := base58.Decode(base58EncodedString)
	if err != nil {
		return errors.Wrap(err, "could not decode base58 encoded string")
	}
	if _, err = serix.DefaultAPI.Decode(context.Background(), decodedBytes, o, serix.WithValidation()); err != nil {
		return errors.Wrap(err, "failed to decode OutputID")
	}

	return nil
}

// FromRandomness generates a random OutputID.
func (o *OutputID) FromRandomness() (err error) {
	if err = o.TransactionID.FromRandomness(); err != nil {
		return errors.Wrap(err, "could not create TransactionID from randomness")
	}

	return nil
}

// Bytes returns a serialized version of the OutputID.
func (o OutputID) Bytes() (serialized []byte, err error) {
	serialized, err = o.TransactionID.Bytes()
	if err != nil {
		return nil, errors.Wrap(err, "could not serialize TransactionID")
	}

	b := make([]byte, serializer.UInt16ByteSize)
	binary.LittleEndian.PutUint16(b, o.Index)

	return byteutils.ConcatBytes(serialized, b), nil
}

// FromBytes un-serializes an OutputID from a []byte.
func (o *OutputID) FromBytes(outputBytes []byte) (consumedBytes int, err error) {
	if consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), outputBytes, o, serix.WithValidation()); err != nil {
		return consumedBytes, errors.Wrap(err, "Fail to parse outputID from bytes")
	}
	return
}

// RegisterAlias allows to register a human-readable alias for the OutputID which will be used as a replacement for the
// String method.
func (o OutputID) RegisterAlias(alias string) {
	_outputIDAliasesMutex.Lock()
	defer _outputIDAliasesMutex.Unlock()

	_outputIDAliases[o] = alias
}

// Alias returns the human-readable alias of the OutputID (or the base58 encoded bytes of no alias was set).
func (o OutputID) Alias() (alias string) {
	_outputIDAliasesMutex.RLock()
	defer _outputIDAliasesMutex.RUnlock()

	if existingAlias, exists := _outputIDAliases[o]; exists {
		return existingAlias
	}

	return fmt.Sprintf("%s, %d", o.TransactionID, int(o.Index))
}

// UnregisterAlias allows to unregister a previously registered alias.
func (o OutputID) UnregisterAlias() {
	_outputIDAliasesMutex.Lock()
	defer _outputIDAliasesMutex.Unlock()

	delete(_outputIDAliases, o)
}

// Base58 returns a base58 encoded version of the OutputID.
func (o OutputID) Base58() (base58Encoded string) {
	return base58.Encode(lo.PanicOnErr(o.Bytes()))
}

// Length returns number of bytes of OutputID.
func (o OutputID) Length() int {
	return o.TransactionID.Length() + serializer.UInt16ByteSize
}

// String returns a human-readable version of the OutputID.
func (o OutputID) String() (humanReadable string) {
	return "OutputID(" + o.Alias() + ")"
}

// EmptyOutputID contains the null-value of the OutputID type.
var EmptyOutputID OutputID

var (
	// _outputIDAliases contains a dictionary of OutputIDs associated to their human-readable alias.
	_outputIDAliases = make(map[OutputID]string)

	// _outputIDAliasesMutex is the mutex that is used to synchronize access to the previous map.
	_outputIDAliasesMutex = sync.RWMutex{}
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputIDs ////////////////////////////////////////////////////////////////////////////////////////////////////

// OutputIDs represents a collection of OutputIDs.
type OutputIDs = *advancedset.AdvancedSet[OutputID]

// NewOutputIDs returns a new OutputID collection with the given elements.
func NewOutputIDs(ids ...OutputID) (newOutputIDs OutputIDs) {
	return advancedset.New(ids...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Outputs //////////////////////////////////////////////////////////////////////////////////////////////////////

// Outputs represents a collection of Output objects indexed by their OutputID.
type Outputs struct {
	// OrderedMap is the underlying data structure that holds the Outputs.
	orderedmap.OrderedMap[OutputID, Output] `serix:"0"`
}

// NewOutputs returns a new Output collection with the given elements.
func NewOutputs(outputs ...Output) (newOutputs *Outputs) {
	newOutputs = &Outputs{*orderedmap.New[OutputID, Output]()}
	for _, output := range outputs {
		newOutputs.Set(output.ID(), output)
	}

	return newOutputs
}

// Add adds the given Output to the collection.
func (o *Outputs) Add(output Output) {
	o.Set(output.ID(), output)
}

// IDs returns the identifiers of the stored Outputs.
func (o *Outputs) IDs() (ids OutputIDs) {
	outputIDs := make([]OutputID, 0)
	o.OrderedMap.ForEach(func(id OutputID, _ Output) bool {
		outputIDs = append(outputIDs, id)
		return true
	})

	return NewOutputIDs(outputIDs...)
}

// ForEach executes the callback for each element in the collection (it aborts if the callback returns an error).
func (o *Outputs) ForEach(callback func(output Output) error) (err error) {
	o.OrderedMap.ForEach(func(_ OutputID, output Output) bool {
		if err = callback(output); err != nil {
			return false
		}

		return true
	})

	return err
}

// String returns a human-readable version of the Outputs.
func (o *Outputs) String() (humanReadable string) {
	structBuilder := stringify.NewStructBuilder("Outputs")
	_ = o.ForEach(func(output Output) error {
		structBuilder.AddField(stringify.NewStructField(output.ID().String(), output))
		return nil
	})

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
