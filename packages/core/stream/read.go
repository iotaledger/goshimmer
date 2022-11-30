package stream

import (
	"encoding/binary"
	"io"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/constraints"
)

// Read reads a generic basic type from the stream.
func Read[T any](reader io.ReadSeeker) (result T, err error) {
	return result, binary.Read(reader, binary.LittleEndian, &result)
}

// ReadSerializable reads a serializable type from the stream (if the serialized field is of fixed size, we can provide
// the length to omit additional information about the length of the serializable).
func ReadSerializable[T any, TPtr constraints.MarshalablePtr[T]](reader io.ReadSeeker, target TPtr, optFixedSize ...int) (err error) {
	var readBytes []byte
	if len(optFixedSize) == 0 {
		if readBytes, err = ReadBlob(reader); err != nil {
			return errors.Errorf("failed to read serialized bytes: %w", err)
		}
	} else {
		if readBytes, err = ReadBytes(reader, uint64(optFixedSize[0])); err != nil {
			return errors.Errorf("failed to read serialized bytes: %w", err)
		}
	}

	if consumedBytes, err := target.FromBytes(readBytes); err != nil {
		return errors.Errorf("failed to parse bytes of serializable: %w", err)
	} else if len(optFixedSize) > 1 && consumedBytes != len(readBytes) {
		return errors.Errorf("failed to parse serializable: consumed bytes (%d) != read bytes (%d)", consumedBytes, len(readBytes))
	}

	return
}

// ReadBytes reads a byte slice of the given size from the stream.
func ReadBytes(reader io.ReadSeeker, size uint64) (bytes []byte, err error) {
	bytes = make([]byte, size)
	if err = binary.Read(reader, binary.LittleEndian, &bytes); err != nil {
		err = errors.Errorf("failed to read %d bytes: %w", size, err)
	}

	return
}

// ReadBlob reads a byte slice from the stream (the first 8 bytes are the length of the blob).
func ReadBlob(reader io.ReadSeeker) (blob []byte, err error) {
	var size uint64
	if size, err = Read[uint64](reader); err != nil {
		err = errors.Errorf("failed to read blob size: %w", err)
	} else if blob, err = ReadBytes(reader, size); err != nil {
		err = errors.Errorf("failed to read blob: %w", err)
	}

	return
}

// ReadCollection reads a collection from the stream (the first 8 bytes are the length of the collection).
func ReadCollection(reader io.ReadSeeker, readCallback func(int) error) (err error) {
	var elementsCount uint64
	if elementsCount, err = Read[uint64](reader); err != nil {
		return errors.Errorf("failed to read collection count: %w", err)
	}

	for i := 0; i < int(elementsCount); i++ {
		if err = readCallback(i); err != nil {
			return errors.Errorf("failed to read element %d: %w", i, err)
		}
	}

	return
}
