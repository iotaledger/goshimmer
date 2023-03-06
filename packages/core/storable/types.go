package storable

import (
	"encoding/binary"

	"github.com/iotaledger/hive.go/constraints"
)

// StructConstraint is a constraint that is used to ensure that the given type is a valid Struct.
type StructConstraint[A any, B constraints.Ptr[A]] interface {
	*A

	InitStruct(B, string) B
	FromBytes([]byte) (int, error)
	Bytes() ([]byte, error)
}

type SerializableInt64 int64

func (s SerializableInt64) Bytes() (data []byte, err error) {
	data = make([]byte, 8)
	binary.LittleEndian.PutUint64(data, uint64(s))
	return
}

func (s *SerializableInt64) FromBytes(data []byte) (consumedBytes int, err error) {
	*s = SerializableInt64(binary.LittleEndian.Uint64(data))
	return 8, nil
}
