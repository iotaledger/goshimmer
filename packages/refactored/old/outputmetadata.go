package old

import (
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"

	branchdag2 "github.com/iotaledger/goshimmer/packages/refactored/ledger/branchdag"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/refactored/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/types/utxo"

	utxo2 "github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region OutputMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

// OutputMetadata contains additional Output information that are derived from the local perception of the node.
type OutputMetadata struct {
	id                      utxo.OutputID
	branchIDs               branchdag.BranchIDs
	branchIDsMutex          sync.RWMutex
	solid                   bool
	solidMutex              sync.RWMutex
	solidificationTime      time.Time
	solidificationTimeMutex sync.RWMutex
	consumerCount           int
	consumerMutex           sync.RWMutex
	gradeOfFinality         gof.GradeOfFinality
	gradeOfFinalityTime     time.Time
	gradeOfFinalityMutex    sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewOutputMetadata creates a new empty OutputMetadata object.
func NewOutputMetadata(outputID utxo.OutputID) *OutputMetadata {
	return &OutputMetadata{
		id:        outputID,
		branchIDs: branchdag.NewBranchIDs(),
	}
}

// FromObjectStorage creates an OutputMetadata from sequences of key and bytes.
func (o *OutputMetadata) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	outputMetadata, err := o.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse OutputMetadata from bytes: %w", err)
	}
	return outputMetadata, err
}

// FromBytes unmarshals an OutputMetadata object from a sequence of bytes.
func (o *OutputMetadata) FromBytes(bytes []byte) (outputMetadata *OutputMetadata, err error) {
	marshalUtil := marshalutil.New(bytes)
	if outputMetadata, err = o.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse OutputMetadata from MarshalUtil: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals an OutputMetadata object using a MarshalUtil (for easier unmarshalling).
func (o *OutputMetadata) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (outputMetadata *OutputMetadata, err error) {
	if outputMetadata = o; outputMetadata == nil {
		outputMetadata = new(OutputMetadata)
	}

	if outputMetadata.id, err = utxo2.OutputIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse OutputID: %w", err)
		return
	}
	if outputMetadata.branchIDs, err = branchdag2.BranchIDsFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchIDs: %w", err)
		return
	}
	if outputMetadata.solid, err = marshalUtil.ReadBool(); err != nil {
		err = errors.Errorf("failed to parse solid flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if outputMetadata.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		err = errors.Errorf("failed to parse solidification time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	consumerCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = errors.Errorf("failed to parse consumer count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	outputMetadata.consumerCount = int(consumerCount)
	gradeOfFinality, err := marshalUtil.ReadUint8()
	if err != nil {
		err = errors.Errorf("failed to parse grade of finality (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	outputMetadata.gradeOfFinality = gof.GradeOfFinality(gradeOfFinality)
	if outputMetadata.gradeOfFinalityTime, err = marshalUtil.ReadTime(); err != nil {
		err = errors.Errorf("failed to parse gradeOfFinality time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	return
}

// ID returns the OutputID of the Output that the OutputMetadata belongs to.
func (o *OutputMetadata) ID() utxo.OutputID {
	return o.id
}

// BranchIDs returns the identifiers of the Branches that the Output was booked in.
func (o *OutputMetadata) BranchIDs() branchdag.BranchIDs {
	o.branchIDsMutex.RLock()
	defer o.branchIDsMutex.RUnlock()

	return o.branchIDs.Clone()
}

// SetBranchIDs sets the identifiers of the Branches that the Output was booked in.
func (o *OutputMetadata) SetBranchIDs(branchIDs branchdag.BranchIDs) (modified bool) {
	o.branchIDsMutex.Lock()
	defer o.branchIDsMutex.Unlock()

	if o.branchIDs.Equals(branchIDs) {
		return false
	}

	o.branchIDs = branchIDs.Clone()
	o.SetModified()
	return true
}

// AddBranchID adds an identifier of the Branch that the Output was booked in.
func (o *OutputMetadata) AddBranchID(branchID branchdag.BranchID) (modified bool) {
	o.branchIDsMutex.Lock()
	defer o.branchIDsMutex.Unlock()

	if o.branchIDs.Contains(branchID) {
		return false
	}

	delete(o.branchIDs, branchdag.MasterBranchID)

	o.branchIDs.Add(branchID)
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
func (o *OutputMetadata) RegisterConsumer(consumer utxo.TransactionID) (previousConsumerCount int) {
	o.consumerMutex.Lock()
	defer o.consumerMutex.Unlock()

	o.consumerCount++
	o.SetModified()

	return
}

// GradeOfFinality returns the grade of finality.
func (o *OutputMetadata) GradeOfFinality() gof.GradeOfFinality {
	o.gradeOfFinalityMutex.RLock()
	defer o.gradeOfFinalityMutex.RUnlock()
	return o.gradeOfFinality
}

// SetGradeOfFinality updates the grade of finality. It returns true if it was modified.
func (o *OutputMetadata) SetGradeOfFinality(gradeOfFinality gof.GradeOfFinality) (modified bool) {
	o.gradeOfFinalityMutex.Lock()
	defer o.gradeOfFinalityMutex.Unlock()

	if o.gradeOfFinality == gradeOfFinality {
		return
	}

	o.gradeOfFinality = gradeOfFinality
	o.gradeOfFinalityTime = clock.SyncedTime()
	o.SetModified()
	modified = true
	return
}

// GradeOfFinalityTime returns the time the Output's gradeOfFinality was set.
func (o *OutputMetadata) GradeOfFinalityTime() time.Time {
	o.gradeOfFinalityMutex.RLock()
	defer o.gradeOfFinalityMutex.RUnlock()
	return o.gradeOfFinalityTime
}

// Bytes marshals the OutputMetadata into a sequence of bytes.
func (o *OutputMetadata) Bytes() []byte {
	return byteutils.ConcatBytes(o.ObjectStorageKey(), o.ObjectStorageValue())
}

// String returns a human readable version of the OutputMetadata.
func (o *OutputMetadata) String() string {
	return stringify.Struct("OutputMetadata",
		stringify.StructField("id", o.ID()),
		stringify.StructField("branchIDs", o.BranchIDs()),
		stringify.StructField("solid", o.Solid()),
		stringify.StructField("solidificationTime", o.SolidificationTime()),
		stringify.StructField("consumerCount", o.ConsumerCount()),
		stringify.StructField("gradeOfFinality", o.GradeOfFinality()),
		stringify.StructField("gradeOfFinalityTime", o.GradeOfFinalityTime()),
	)
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
		Write(o.BranchIDs()).
		WriteBool(o.Solid()).
		WriteTime(o.SolidificationTime()).
		WriteUint64(uint64(o.ConsumerCount())).
		WriteUint8(uint8(o.GradeOfFinality())).
		WriteTime(o.GradeOfFinalityTime()).
		Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = new(OutputMetadata)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsMetadata //////////////////////////////////////////////////////////////////////////////////////////////

// OutputsMetadata represents a list of OutputMetadata objects.
type OutputsMetadata []*OutputMetadata

// OutputIDs returns the OutputIDs of the Outputs in the list.
func (o OutputsMetadata) OutputIDs() (outputIDs []utxo.OutputID) {
	outputIDs = make([]utxo.OutputID, len(o))
	for i, outputMetadata := range o {
		outputIDs[i] = outputMetadata.ID()
	}

	return
}

// ConflictIDs returns the ConflictIDs that are the equivalent of the OutputIDs in the list.
func (o OutputsMetadata) ConflictIDs() (conflictIDs branchdag.ConflictIDs) {
	conflictIDsSlice := make([]branchdag.ConflictID, len(o))
	for i, input := range o {
		conflictIDsSlice[i] = branchdag2.NewConflictID(input.ID())
	}
	conflictIDs = branchdag.NewConflictIDs(conflictIDsSlice...)

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

// SpentOutputsMetadata returns the spent elements of the list of OutputsMetadata objects.
func (o OutputsMetadata) SpentOutputsMetadata() (spentOutputsMetadata OutputsMetadata) {
	spentOutputsMetadata = make(OutputsMetadata, 0)
	for _, inputMetadata := range o {
		if inputMetadata.ConsumerCount() >= 1 {
			spentOutputsMetadata = append(spentOutputsMetadata, inputMetadata)
		}
	}

	return spentOutputsMetadata
}

// String returns a human-readable version of the OutputsMetadata.
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
type OutputsMetadataByID map[utxo.OutputID]*OutputMetadata

// IDs returns the OutputIDs that are used as keys in the collection.
func (o OutputsMetadataByID) IDs() (outputIDs []utxo.OutputID) {
	outputIDs = make([]utxo.OutputID, 0, len(o))
	for outputID := range o {
		outputIDs = append(outputIDs, outputID)
	}

	return
}

// ConflictIDs turns the list of OutputMetadata objects into their corresponding ConflictIDs.
func (o OutputsMetadataByID) ConflictIDs() (conflictIDs branchdag.ConflictIDs) {
	conflictIDsSlice := make([]branchdag.ConflictID, 0, len(o))
	for inputID := range o {
		conflictIDsSlice = append(conflictIDsSlice, branchdag2.NewConflictID(inputID))
	}
	conflictIDs = branchdag.NewConflictIDs(conflictIDsSlice...)

	return
}

// Filter returns the OutputsMetadataByID that are sharing a set membership with the given Inputs.
func (o OutputsMetadataByID) Filter(outputIDsToInclude []utxo.OutputID) (intersectionOfInputs OutputsMetadataByID) {
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
