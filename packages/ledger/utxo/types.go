package utxo

import (
	"context"
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/orderedmap"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"
)

// region TransactionID ////////////////////////////////////////////////////////////////////////////////////////////////

// TransactionID is a unique identifier for a Transaction.
type TransactionID struct {
	types.Identifier `serix:"0"`
}

// NewTransactionID returns a new TransactionID for the given data.
func NewTransactionID(txData []byte) (new TransactionID) {
	return TransactionID{
		types.NewIdentifier(txData),
	}
}

// Unmarshal un-serializes a TransactionID using a MarshalUtil.
func (t TransactionID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (txID TransactionID, err error) {
	err = txID.Identifier.FromMarshalUtil(marshalUtil)
	return
}

// String returns a human-readable version of the TransactionID.
func (t TransactionID) String() (humanReadable string) {
	return "TransactionID(" + t.Alias() + ")"
}

// EmptyTransactionID contains the null-value of the TransactionID type.
var EmptyTransactionID TransactionID

// TransactionIDLength contains the byte length of a serialized TransactionID.
const TransactionIDLength = types.IdentifierLength

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionIDs ///////////////////////////////////////////////////////////////////////////////////////////////

// TransactionIDs represents a collection of TransactionIDs.
type TransactionIDs = *set.AdvancedSet[TransactionID]

// NewTransactionIDs returns a new TransactionID collection with the given elements.
func NewTransactionIDs(ids ...TransactionID) (new TransactionIDs) {
	return set.NewAdvancedSet[TransactionID](ids...)
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
		return errors.Errorf("could not decode base58 encoded string: %w", err)
	}
	_, err = o.FromBytes(decodedBytes)
	return err
}

// FromRandomness generates a random OutputID.
func (o *OutputID) FromRandomness() (err error) {
	if err = o.TransactionID.FromRandomness(); err != nil {
		return errors.Errorf("could not create TransactionID from randomness: %w", err)
	}

	return nil
}

// FromMarshalUtil un-serializes an OutputID from a MarshalUtil.
func (o *OutputID) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if err = o.TransactionID.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse TransactionID: %w", err)
	}
	if o.Index, err = marshalUtil.ReadUint16(); err != nil {
		return errors.Errorf("failed to parse Index: %w", err)
	}

	return nil
}

// FromBytes unmarshals an OutputID from a sequence of bytes.
func (o *OutputID) FromBytes(data []byte) (consumedBytes int, err error) {
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), data, o, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse OutputID: %w", err)
		return
	}
	return
}

// Unmarshal un-serializes a OutputID using a MarshalUtil (additional unmarshal signature required for AdvancedSet).
func (o OutputID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (outputID OutputID, err error) {
	err = outputID.FromMarshalUtil(marshalUtil)
	return outputID, err
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
	return base58.Encode(o.Bytes())
}

// Bytes returns the serialized version of the OutputID.
func (o OutputID) Bytes() (serialized []byte) {
	return marshalutil.New().
		Write(o.TransactionID).
		WriteUint16(o.Index).
		Bytes()
}

// String returns a human-readable version of the OutputID.
func (o OutputID) String() (humanReadable string) {
	return "OutputID(" + o.Alias() + ")"
}

// EmptyOutputID contains the null-value of the OutputID type.
var EmptyOutputID OutputID

// OutputIDLength contains the byte length of a serialized OutputID.
const OutputIDLength = TransactionIDLength + 2

var (
	// _outputIDAliases contains a dictionary of OutputIDs associated to their human-readable alias.
	_outputIDAliases = make(map[OutputID]string)

	// _outputIDAliasesMutex is the mutex that is used to synchronize access to the previous map.
	_outputIDAliasesMutex = sync.RWMutex{}
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputIDs ////////////////////////////////////////////////////////////////////////////////////////////////////

// OutputIDs represents a collection of OutputIDs.
type OutputIDs = *set.AdvancedSet[OutputID]

// NewOutputIDs returns a new OutputID collection with the given elements.
func NewOutputIDs(ids ...OutputID) (new OutputIDs) {
	return set.NewAdvancedSet[OutputID](ids...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Outputs //////////////////////////////////////////////////////////////////////////////////////////////////////

// Outputs represents a collection of Output objects indexed by their OutputID.
type Outputs struct {
	// OrderedMap is the underlying data structure that holds the Outputs.
	*orderedmap.OrderedMap[OutputID, Output]
}

// NewOutputs returns a new Output collection with the given elements.
func NewOutputs(outputs ...Output) (new Outputs) {
	new = Outputs{orderedmap.New[OutputID, Output]()}
	for _, output := range outputs {
		new.Set(output.ID(), output)
	}

	return new
}

func (o *Outputs) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil, outputFactory OutputFactory) (err error) {
	if o.OrderedMap == nil {
		o.OrderedMap = orderedmap.New[OutputID, Output]()
	}

	outputCount, err := marshalUtil.ReadUint64()
	if err != nil {
		return errors.Errorf("unable to read output count: %w", err)
	}

	for i := uint64(0); i < outputCount; i++ {
		var outputID OutputID
		if _, err = outputID.FromBytes(marshalUtil.Bytes()); err != nil {
			return errors.Errorf("unable to read output ID: %w", err)
		}

		output, outputErr := outputFactory(marshalUtil)
		if outputErr != nil {
			return errors.Errorf("unable to read output: %w", outputErr)
		}
		output.SetID(outputID)

		o.Add(output)
	}

	return nil
}

// Add adds the given Output to the collection.
func (o Outputs) Add(output Output) {
	o.Set(output.ID(), output)
}

// IDs returns the identifiers of the stored Outputs.
func (o Outputs) IDs() (ids OutputIDs) {
	outputIDs := make([]OutputID, 0)
	o.OrderedMap.ForEach(func(id OutputID, _ Output) bool {
		outputIDs = append(outputIDs, id)
		return true
	})

	return NewOutputIDs(outputIDs...)
}

// ForEach executes the callback for each element in the collection (it aborts if the callback returns an error).
func (o Outputs) ForEach(callback func(output Output) error) (err error) {
	o.OrderedMap.ForEach(func(_ OutputID, output Output) bool {
		if err = callback(output); err != nil {
			return false
		}

		return true
	})

	return err
}

// Bytes returns a serialized version of the Outputs.
func (o Outputs) Bytes() (serialized []byte) {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteUint64(uint64(o.Size()))
	_ = o.ForEach(func(output Output) error {
		marshalUtil.Write(output.ID())
		marshalUtil.Write(output)
		return nil
	})

	return marshalUtil.Bytes()
}

// Strings returns a human-readable version of the Outputs.
func (o Outputs) String() (humanReadable string) {
	structBuilder := stringify.StructBuilder("Outputs")
	_ = o.ForEach(func(output Output) error {
		structBuilder.AddField(stringify.StructField(output.ID().String(), output))
		return nil
	})

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
