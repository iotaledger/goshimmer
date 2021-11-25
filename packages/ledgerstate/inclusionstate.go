package ledgerstate

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
)

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
