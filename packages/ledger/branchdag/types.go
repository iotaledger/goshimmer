package branchdag

import (
	"fmt"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// region BranchID /////////////////////////////////////////////////////////////////////////////////////////////////////

type BranchID struct {
	types.Identifier
}

func NewBranchID(txID utxo.TransactionID) (new BranchID) {
	return BranchID{txID.Identifier}
}

// Unmarshal unmarshals a BranchID using a MarshalUtil (for easier unmarshalling).
func (t BranchID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (branchID BranchID, err error) {
	err = branchID.Identifier.FromMarshalUtil(marshalUtil)
	return
}

func (t BranchID) String() (humanReadable string) {
	return "BranchID(" + t.Alias() + ")"
}

var MasterBranchID BranchID

const BranchIDLength = types.IdentifierLength

func init() {
	MasterBranchID.RegisterAlias("MasterBranch")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchIDs ////////////////////////////////////////////////////////////////////////////////////////////////////

type BranchIDs = *set.AdvancedSet[BranchID]

func NewBranchIDs(ids ...BranchID) (new BranchIDs) {
	return set.NewAdvancedSet[BranchID](ids...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictID ///////////////////////////////////////////////////////////////////////////////////////////////////

const ConflictIDLength = types.IdentifierLength

type ConflictID struct {
	types.Identifier
}

func NewConflictID(outputID utxo.OutputID) (new ConflictID) {
	return ConflictID{outputID.Identifier}
}

// Unmarshal unmarshals a ConflictID using a MarshalUtil (for easier unmarshalling).
func (t ConflictID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (conflictID ConflictID, err error) {
	err = conflictID.Identifier.FromMarshalUtil(marshalUtil)
	return
}

func (t ConflictID) String() (humanReadable string) {
	return "ConflictID(" + t.Alias() + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictIDs //////////////////////////////////////////////////////////////////////////////////////////////////

type ConflictIDs = *set.AdvancedSet[ConflictID]

func NewConflictIDs(ids ...ConflictID) (new ConflictIDs) {
	return set.NewAdvancedSet[ConflictID](ids...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ArithmeticBranchIDs //////////////////////////////////////////////////////////////////////////////////////////

// ArithmeticBranchIDs represents an arithmetic collection of BranchIDs that allows us to add and subtract them from
// each other.
type ArithmeticBranchIDs map[BranchID]int

// NewArithmeticBranchIDs returns a new ArithmeticBranchIDs object.
func NewArithmeticBranchIDs(optionalBranchIDs ...BranchIDs) (newArithmeticBranchIDs ArithmeticBranchIDs) {
	newArithmeticBranchIDs = make(ArithmeticBranchIDs)
	if len(optionalBranchIDs) >= 1 {
		newArithmeticBranchIDs.Add(optionalBranchIDs[0])
	}

	return newArithmeticBranchIDs
}

// Add adds all BranchIDs to the collection.
func (a ArithmeticBranchIDs) Add(branchIDs BranchIDs) {
	_ = branchIDs.ForEach(func(branchID BranchID) (err error) {
		a[branchID]++
		return nil
	})
}

// Subtract subtracts all BranchIDs from the collection.
func (a ArithmeticBranchIDs) Subtract(branchIDs BranchIDs) {
	_ = branchIDs.ForEach(func(branchID BranchID) (err error) {
		a[branchID]--
		return nil
	})
}

// BranchIDs returns the BranchIDs represented by this collection.
func (a ArithmeticBranchIDs) BranchIDs() (branchIDs BranchIDs) {
	branchIDs = NewBranchIDs()
	for branchID, value := range a {
		if value >= 1 {
			branchIDs.Add(branchID)
		}
	}

	return
}

// String returns a human-readable version of the ArithmeticBranchIDs.
func (a ArithmeticBranchIDs) String() string {
	if len(a) == 0 {
		return "ArithmeticBranchIDs() = " + a.BranchIDs().String()
	}

	result := "ArithmeticBranchIDs("
	i := 0
	for branchID, value := range a {
		switch {
		case value == 1:
			if i != 0 {
				result += " + "
			}

			result += branchID.String()
			i++
		case value > 1:
			if i != 0 {
				result += " + "
			}

			result += strconv.Itoa(value) + "*" + branchID.String()
			i++
		case value == 0:
		case value == -1:
			if i != 0 {
				result += " - "
			} else {
				result += "-"
			}

			result += branchID.String()
			i++
		case value < -1:
			if i != 0 {
				result += " - "
			}

			result += strconv.Itoa(-value) + "*" + branchID.String()
			i++
		}
	}
	result += ") = " + a.BranchIDs().String()

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region InclusionState ///////////////////////////////////////////////////////////////////////////////////////////////

// InclusionState represents the confirmation status of elements in the ledger.
type InclusionState uint8

const (
	// Pending represents elements that have neither been confirmed nor rejected.
	Pending InclusionState = iota

	// Confirmed represents elements that have been confirmed and will stay part of the ledger state forever.
	Confirmed

	// Rejected represents elements that have been rejected and will not be included in the ledger state.
	Rejected
)

// InclusionStateFromBytes unmarshals an InclusionState from a sequence of bytes.
func InclusionStateFromBytes(bytes []byte) (inclusionState InclusionState, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if inclusionState, err = InclusionStateFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse InclusionState from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// InclusionStateFromMarshalUtil unmarshals an InclusionState from using a MarshalUtil (for easier unmarshalling).
func InclusionStateFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (inclusionState InclusionState, err error) {
	untypedInclusionState, err := marshalUtil.ReadUint8()
	if err != nil {
		err = errors.Errorf("failed to parse InclusionState (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	if inclusionState = InclusionState(untypedInclusionState); inclusionState > Rejected {
		err = errors.Errorf("invalid %s: %w", inclusionState, cerrors.ErrParseBytesFailed)
	}

	return
}

// String returns a human-readable representation of the InclusionState.
func (i InclusionState) String() string {
	switch i {
	case Pending:
		return "InclusionState(Pending)"
	case Confirmed:
		return "InclusionState(Confirmed)"
	case Rejected:
		return "InclusionState(Rejected)"
	default:
		return fmt.Sprintf("InclusionState(%X)", uint8(i))
	}
}

// Bytes returns a marshaled representation of the InclusionState.
func (i InclusionState) Bytes() []byte {
	return marshalutil.New(marshalutil.Uint8Size).WriteUint8(uint8(i)).Bytes()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
