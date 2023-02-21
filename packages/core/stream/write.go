package stream

import (
	"encoding/binary"
	"io"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/constraints"
)

// Write writes a generic basic type from the stream.
func Write[T any](writer io.WriteSeeker, value T) (err error) {
	return binary.Write(writer, binary.LittleEndian, value)
}

// WriteSerializable writes a serializable type to the stream (if the serialized field is of fixed size, we can provide
// the length to omit additional information about the length of the serializable).
func WriteSerializable[T constraints.Serializable](writer io.WriteSeeker, target T, optFixedSize ...int) (err error) {
	serializedBytes, err := target.Bytes()
	if err != nil {
		return errors.Wrap(err, "failed to serialize target")
	}

	if len(optFixedSize) == 0 {
		if err = WriteBlob(writer, serializedBytes); err != nil {
			return errors.Wrap(err, "failed to write serialized bytes")
		}

		return
	}

	if len(serializedBytes) != optFixedSize[0] {
		return errors.Errorf("serialized bytes length (%d) != fixed size (%d)", len(serializedBytes), optFixedSize[0])
	} else if err = Write(writer, serializedBytes); err != nil {
		return errors.Wrap(err, "failed to write target")
	}

	return
}

// WriteBlob writes a byte slice to the stream (the first 8 bytes are the length of the blob).
func WriteBlob(writer io.WriteSeeker, blob []byte) (err error) {
	if err = Write(writer, uint64(len(blob))); err != nil {
		err = errors.Wrap(err, "failed to write blob length")
	} else if err = Write(writer, blob); err != nil {
		err = errors.Wrap(err, "failed to write blob")
	}

	return
}

// WriteCollection writes a collection to the stream (the first 8 bytes are the length of the collection).
func WriteCollection(writer io.WriteSeeker, writeCollection func() (elementsCount uint64, err error)) (err error) {
	var elementsCount uint64
	var startOffset, endOffset int64
	if startOffset, err = Offset(writer); err != nil {
		err = errors.Wrap(err, "failed to get start offset")
	} else if _, err = Skip(writer, 8); err != nil {
		err = errors.Wrap(err, "failed to skip elements count")
	} else if elementsCount, err = writeCollection(); err != nil {
		err = errors.Wrap(err, "failed to write collection")
	} else if endOffset, err = Offset(writer); err != nil {
		err = errors.Wrap(err, "failed to read end offset of collection")
	} else if _, err = GoTo(writer, startOffset); err != nil {
		err = errors.Wrap(err, "failed to seek to start of attestors")
	} else if err = Write(writer, elementsCount); err != nil {
		err = errors.Wrap(err, "failed to write attestors count")
	} else if _, err = GoTo(writer, endOffset); err != nil {
		err = errors.Wrap(err, "failed to seek to end of attestors")
	}

	return
}
