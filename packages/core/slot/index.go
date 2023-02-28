package slot

import (
	"context"
	"fmt"

	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

// Index is the ID of a slot.
type Index int64

func IndexFromBytes(bytes []byte) (ei Index, consumedBytes int, err error) {
	if consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, &ei); err != nil {
		panic(err)
	}

	return
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

// Max returns the maximum of the two given slots.
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
