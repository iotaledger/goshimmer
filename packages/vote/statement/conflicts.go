package statement

import (
	"fmt"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/marshalutil"
	"golang.org/x/xerrors"
)

const (
	// ConflictLength defines the Conflict length in bytes.
	ConflictLength = transaction.IDLength + 1
)

// Conflict holds the conflicting transaction ID and its opinion.
type Conflict struct {
	ID      transaction.ID
	Opinion bool
}

// Conflicts is a slice of Conflict.
type Conflicts []Conflict

// Bytes returns the conflict statment encoded as bytes.
func (c Conflict) Bytes() (bytes []byte) {
	bytes = make([]byte, ConflictLength)

	// initialize helper
	marshalUtil := marshalutil.New(bytes)
	marshalUtil.WriteBytes(c.ID.Bytes())
	marshalUtil.WriteBool(c.Opinion)

	return bytes
}

// Bytes returns the conflicts statments encoded as bytes.
func (c Conflicts) Bytes() (bytes []byte) {
	bytes = make([]byte, ConflictLength*len(c))

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	for _, conflict := range c {
		marshalUtil.WriteBytes(conflict.ID.Bytes())
		marshalUtil.WriteBool(conflict.Opinion)
	}

	return bytes
}

// ConflictFromBytes parses a conflict statement from a byte slice.
func ConflictFromBytes(bytes []byte) (result Conflict, consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read information that are required to identify the Conflict
	result = Conflict{}
	bytesID, err := marshalUtil.ReadBytes(int(transaction.IDLength))
	if err != nil {
		err = xerrors.Errorf("failed to parse ID from conflict: %w", err)
		return
	}
	result.ID, _, err = transaction.IDFromBytes(bytesID)
	if err != nil {
		err = xerrors.Errorf("failed to parse ID from bytes: %w", err)
		return
	}
	if result.Opinion, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse opinion from conflict: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ConflictsFromBytes parses a slice of conflict statements from a byte slice.
func ConflictsFromBytes(bytes []byte, n int) (result Conflicts, consumedBytes int, err error) {
	if len(bytes)/ConflictLength < n {
		err = xerrors.Errorf("not enough bytes to parse %d conflicts", n)
		return
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	result = Conflicts{}
	for i := 0; i < n; i++ {
		next, e := marshalUtil.ReadBytes(ConflictLength)
		if e != nil {
			err = xerrors.Errorf("failed to read bytes while parsing conflict from conflicts: %w", e)
			return
		}
		conflict, _, e := ConflictFromBytes(next)
		if e != nil {
			err = xerrors.Errorf("failed to parse conflict from conflicts: %w", e)
			return
		}
		result = append(result, conflict)
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (c Conflicts) String() (result string) {
	for _, conflict := range c {
		result += fmt.Sprintln(conflict.ID, conflict.Opinion)
	}
	return result
}
