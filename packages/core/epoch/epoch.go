package epoch

import (
	"context"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

var (
	// GenesisTime is the time (Unix in seconds) of the genesis.
	GenesisTime int64 = 1666037699

	// Duration is the default epoch duration in seconds.
	Duration int64 = 10
)

// region Index ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Index is the ID of an epoch.
type Index int64

func IndexFromBytes(bytes []byte) (ei Index, consumedBytes int, err error) {
	if consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, &ei); err != nil {
		panic(err)
	}

	return
}

// IndexFromTime calculates the Index from the given time.
//
// Note: Epochs are counted starting from 1 because 0 is reserved for the genesis which has to be addressable as its own
// epoch as part of the commitment chains.
func IndexFromTime(t time.Time) Index {
	elapsedSeconds := t.Unix() - GenesisTime
	if elapsedSeconds < 0 {
		return 0
	}

	return Index(elapsedSeconds/Duration + 1)
}

func (i Index) Bytes() []byte {
	bytes, err := serix.DefaultAPI.Encode(context.Background(), i, serix.WithValidation())
	if err != nil {
		panic(err)
	}

	return bytes
}

func (i Index) Length() int {
	return 8
}

func (i Index) String() string {
	return fmt.Sprintf("Index(%d)", i)
}

// StartTime calculates the start time of the given epoch.
func (i Index) StartTime() time.Time {
	startUnix := GenesisTime + int64(i-1)*Duration
	return time.Unix(startUnix, 0)
}

// EndTime calculates the end time of the given epoch.
func (i Index) EndTime() time.Time {
	endUnix := GenesisTime + int64(i-1)*Duration + Duration - 1
	return time.Unix(endUnix, 0)
}

// Max returns the maximum of the two given epochs.
func (i Index) Max(other Index) Index {
	if i > other {
		return i
	}

	return other
}

// Abs returns the absolute value of the Index.
func (i Index) Abs() (absolute Index) {
	if i < 0 {
		return -i
	}

	return i
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region IndexedID ////////////////////////////////////////////////////////////////////////////////////////////////////

type IndexedID interface {
	comparable

	Index() Index
	String() string
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region IndexedEntity ////////////////////////////////////////////////////////////////////////////////////////////////

type IndexedEntity[IDType IndexedID] interface {
	ID() IDType
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
