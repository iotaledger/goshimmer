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

// BranchID is a unique identifier for a Branch.
type BranchID struct {
	types.Identifier
}

// NewBranchID returns a new BranchID from the given TransactionID.
func NewBranchID(txID utxo.TransactionID) (new BranchID) {
	return BranchID{txID.Identifier}
}

// TransactionID returns the TransactionID from the BranchID.
func (b BranchID) TransactionID() utxo.TransactionID {
	return utxo.TransactionID{Identifier: b.Identifier}
}

// Unmarshal un-serializes a BranchID using a MarshalUtil.
func (b BranchID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (branchID BranchID, err error) {
	err = branchID.Identifier.FromMarshalUtil(marshalUtil)
	return
}

// String returns a human-readable version of the BranchID.
func (b BranchID) String() (humanReadable string) {
	return "BranchID(" + b.Alias() + ")"
}

// UndefinedBranchID contains the null-value of the BranchID type.
var UndefinedBranchID BranchID

// MasterBranchID contains the identifier of the MasterBranch.
var MasterBranchID = BranchID{types.Identifier{1}}

// BranchIDLength contains the byte length of a serialized BranchID.
const BranchIDLength = types.IdentifierLength

// init is used to register a human-readable alias for the MasterBranchID.
func init() {
	MasterBranchID.RegisterAlias("MasterBranchID")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchIDs ////////////////////////////////////////////////////////////////////////////////////////////////////

// BranchIDs represents a collection of BranchIDs.
type BranchIDs = *set.AdvancedSet[BranchID]

// NewBranchIDs returns a new BranchID collection with the given elements.
func NewBranchIDs(ids ...BranchID) (new BranchIDs) {
	return set.NewAdvancedSet[BranchID](ids...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ArithmeticBranchIDs //////////////////////////////////////////////////////////////////////////////////////////

// ArithmeticBranchIDs represents a set of BranchIDs that allows to perform basic arithmetic set operations (like
// addition or subtraction).
//
// It keeps track of the exact amount of occurrences of each Branch, so adding the same Branch twice will result in the
// Branch existing twice in the set.
type ArithmeticBranchIDs map[BranchID]int

// NewArithmeticBranchIDs returns a new ArithmeticBranchIDs object optionally initialized with the named BranchIDs.
func NewArithmeticBranchIDs(optionalBranchIDs ...BranchIDs) (new ArithmeticBranchIDs) {
	new = make(ArithmeticBranchIDs)
	for _, branchIDs := range optionalBranchIDs {
		new.Add(branchIDs)
	}

	return new
}

// Add adds the BranchIDs to the set.
func (a ArithmeticBranchIDs) Add(branchIDs BranchIDs) {
	for it := branchIDs.Iterator(); it.HasNext(); {
		a[it.Next()]++
	}
}

// Subtract removes the BranchIDs from the set.
func (a ArithmeticBranchIDs) Subtract(branchIDs BranchIDs) {
	for it := branchIDs.Iterator(); it.HasNext(); {
		a[it.Next()]--
	}
}

// BranchIDs returns the BranchIDs with a positive amount of occurrences.
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

// region ConflictID ///////////////////////////////////////////////////////////////////////////////////////////////////

// ConflictID is a unique identifier for a Conflict.
type ConflictID struct {
	types.Identifier
}

// NewConflictID returns a new ConflictID from the given OutputID.
func NewConflictID(outputID utxo.OutputID) (new ConflictID) {
	return ConflictID{outputID.Identifier}
}

// OutputID returns the OutputID from the ConflictID.
func (c ConflictID) OutputID() utxo.OutputID {
	return utxo.OutputID{Identifier: c.Identifier}
}

// Unmarshal un-serializes a ConflictID using a MarshalUtil.
func (c ConflictID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (conflictID ConflictID, err error) {
	err = conflictID.Identifier.FromMarshalUtil(marshalUtil)
	return
}

// String returns a human-readable version of the ConflictID.
func (c ConflictID) String() (humanReadable string) {
	return "ConflictID(" + c.Alias() + ")"
}

// ConflictIDLength contains the byte length of a serialized ConflictID.
const ConflictIDLength = types.IdentifierLength

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictIDs //////////////////////////////////////////////////////////////////////////////////////////////////

// ConflictIDs represents a collection of ConflictIDs.
type ConflictIDs = *set.AdvancedSet[ConflictID]

// NewConflictIDs returns a new ConflictID collection with the given elements.
func NewConflictIDs(ids ...ConflictID) (new ConflictIDs) {
	return set.NewAdvancedSet[ConflictID](ids...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region InclusionState ///////////////////////////////////////////////////////////////////////////////////////////////

// InclusionState represents the confirmation status of branches in the BranchDAG.
type InclusionState uint8

// FromMarshalUtil un-serializes an InclusionState using a MarshalUtil.
func (i *InclusionState) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	untypedInclusionState, err := marshalUtil.ReadUint8()
	if err != nil {
		return errors.Errorf("failed to parse InclusionState (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if *i = InclusionState(untypedInclusionState); *i > Rejected {
		return errors.Errorf("invalid %s: %w", *i, cerrors.ErrParseBytesFailed)
	}

	return nil
}

// String returns a human-readable version of the InclusionState.
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

// Bytes returns a serialized version of the InclusionState.
func (i InclusionState) Bytes() []byte {
	return marshalutil.New(marshalutil.Uint8Size).WriteUint8(uint8(i)).Bytes()
}

const (
	// Pending represents elements that have neither been confirmed nor rejected.
	Pending InclusionState = iota

	// Confirmed represents elements that have been confirmed and will stay part of the ledger state forever.
	Confirmed

	// Rejected represents elements that have been rejected and will not be included in the ledger state.
	Rejected
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
