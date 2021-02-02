package ledgerstate

import (
	"fmt"

	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"golang.org/x/xerrors"
)

const (
	// Pending represents the state of Transactions and Branches that have not been assigned a final state regarding
	// their inclusion in the ledger state.
	Pending InclusionState = iota

	// Confirmed represents the state of Transactions and Branches that have been accepted to be part of the ledger
	// state.
	Confirmed

	// Rejected represents the state of Transactions and Branches that have been rejected to be part of the ledger
	// state.
	Rejected
)

// InclusionState represents a type that encodes if a Transaction or Branch has been included in the ledger state.
type InclusionState uint8

// InclusionStateFromBytes unmarshals an InclusionState from a sequence of bytes.
func InclusionStateFromBytes(inclusionStateBytes []byte) (inclusionState InclusionState, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(inclusionStateBytes)
	if inclusionState, err = InclusionStateFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse InclusionState from MarshalUtil: %w", err)
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// InclusionStateFromMarshalUtil unmarshals an InclusionState using a MarshalUtil (for easier unmarshaling).
func InclusionStateFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (inclusionState InclusionState, err error) {
	inclusionStateUint8, err := marshalUtil.ReadUint8()
	if err != nil {
		err = xerrors.Errorf("failed to parse InclusionState (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	switch inclusionState = InclusionState(inclusionStateUint8); inclusionState {
	case Pending:
	case Confirmed:
	case Rejected:
	default:
		err = xerrors.Errorf("unsupported InclusionState (%X): %w", inclusionStateUint8, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// Bytes returns a marshaled version of the InclusionState.
func (i InclusionState) Bytes() []byte {
	return []byte{byte(i)}
}

// String returns a human readable version of the InclusionState.
func (i InclusionState) String() string {
	inclusionStateNames := [...]string{
		"Pending",
		"Confirmed",
		"Rejected",
	}

	if int(i) > len(inclusionStateNames) {
		return fmt.Sprintf("InclusionState(%X)", byte(i))
	}

	return "InclusionState(" + inclusionStateNames[i] + ")"
}
