package branchdag

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
)

type ConflictIDType[T any] interface {
	comparable

	Unmarshal(marshalUtil *marshalutil.MarshalUtil) (conflict T, err error)
	Bytes() (serialized []byte)
	Base58() (base58Encoded string)
	String() (humanReadable string)
}

type ConflictSetIDType[T any] interface {
	comparable

	Unmarshal(marshalUtil *marshalutil.MarshalUtil) (conflictSet T, err error)
	Bytes() (serialized []byte)
	Base58() (base58Encoded string)
	String() (humanReadable string)
}

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
