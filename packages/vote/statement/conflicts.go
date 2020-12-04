package statement

import (
	"fmt"
	"strconv"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

const (
	// ConflictLength defines the Conflict length in bytes.
	ConflictLength = transaction.IDLength + OpinionLength
)

// region Conflict /////////////////////////////////////////////////////////////////////////////////////////////////////

// Conflict holds the conflicting transaction ID and its opinion.
type Conflict struct {
	ID transaction.ID
	Opinion
}

// Bytes returns the conflict statement encoded as bytes.
func (c Conflict) Bytes() (bytes []byte) {
	return marshalutil.New(ConflictLength).
		Write(c.ID).
		Write(c.Opinion).
		Bytes()
}

// ConflictFromBytes parses a conflict statement from a byte slice.
func ConflictFromBytes(bytes []byte) (conflict Conflict, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if conflict, err = ConflictFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Conflict from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ConflictFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func ConflictFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (conflict Conflict, err error) {
	readStartOffset := marshalUtil.ReadOffset()

	conflict = Conflict{}
	bytesID, err := marshalUtil.ReadBytes(int(transaction.IDLength))
	if err != nil {
		err = xerrors.Errorf("failed to parse ID from conflict: %w", err)
		return
	}
	conflict.ID, _, err = transaction.IDFromBytes(bytesID)
	if err != nil {
		err = xerrors.Errorf("failed to parse ID from bytes: %w", err)
		return
	}

	conflict.Opinion, err = OpinionFromMarshalUtil(marshalUtil)
	if err != nil {
		err = xerrors.Errorf("failed to parse opinion from conflict: %w", err)
		return
	}

	// return the number of bytes we processed
	parsedBytes := marshalUtil.ReadOffset() - readStartOffset
	if parsedBytes != ConflictLength {
		err = xerrors.Errorf("parsed bytes (%d) did not match expected size (%d): %w", parsedBytes, ConflictLength, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// String returns a human readable version of the Conflict.
func (c Conflict) String() string {
	structBuilder := stringify.StructBuilder("Conflict:")
	structBuilder.AddField(stringify.StructField("ID", c.ID.String()))
	structBuilder.AddField(stringify.StructField("Opinion", c.Opinion))

	return structBuilder.String()
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////////

// region Conflicts /////////////////////////////////////////////////////////////////////////////////////////////////////

// Conflicts is a slice of Conflict.
type Conflicts []Conflict

// Bytes returns the conflicts statements encoded as bytes.
func (c Conflicts) Bytes() (bytes []byte) {
	// initialize helper
	marshalUtil := marshalutil.New(ConflictLength * len(c))

	for _, conflict := range c {
		marshalUtil.Write(conflict)
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the Conflicts.
func (c Conflicts) String() string {
	structBuilder := stringify.StructBuilder("Conflicts")
	for i, conflict := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), conflict))
	}

	return structBuilder.String()
}

// ConflictsFromBytes parses a slice of conflict statements from a byte slice.
func ConflictsFromBytes(bytes []byte, n uint32) (conflicts Conflicts, consumedBytes int, err error) {
	if len(bytes)/ConflictLength < int(n) {
		err = xerrors.Errorf("not enough bytes to parse %d conflicts", n)
		return
	}

	marshalUtil := marshalutil.New(bytes)
	if conflicts, err = ConflictsFromMarshalUtil(marshalUtil, n); err != nil {
		err = xerrors.Errorf("failed to parse Conflicts from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ConflictsFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func ConflictsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil, n uint32) (conflicts Conflicts, err error) {
	readStartOffset := marshalUtil.ReadOffset()

	conflicts = Conflicts{}
	for i := 0; i < int(n); i++ {
		conflict, e := ConflictFromMarshalUtil(marshalUtil)
		if e != nil {
			err = fmt.Errorf("failed to parse conflict from marshalutil: %w", e)
			return
		}
		conflicts = append(conflicts, conflict)
	}

	// return the number of bytes we processed
	parsedBytes := marshalUtil.ReadOffset() - readStartOffset
	if parsedBytes != int(ConflictLength*n) {
		err = xerrors.Errorf("parsed bytes (%d) did not match expected size (%d): %w", parsedBytes, ConflictLength*n, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////////
