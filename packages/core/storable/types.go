package storable

import (
	"encoding/binary"

	"github.com/iotaledger/hive.go/core/generics/constraints"
)

// StructConstraint is a constraint that is used to ensure that the given type is a valid Struct.
type StructConstraint[A any, B constraints.Ptr[A]] interface {
	*A

	InitStruct(B, string) B
	FromBytes([]byte) (int, error)
	Bytes() ([]byte, error)
}

type serializable[A any] interface {
	*A

	Bytes() ([]byte, error)
	FromBytes([]byte) (int, error)
}

type SerializableUint64 uint64

func (s SerializableUint64) Bytes() (data []byte, err error) {
	data = make([]byte, 8)
	binary.PutUvarint(data, uint64(s))
	return
}

func (s *SerializableUint64) FromBytes(data []byte) (consumedBytes int, err error) {
	*s = SerializableUint64(uint64(binary.LittleEndian.Uint64(data)))
	return 8, nil
}
