package stream

import (
	"encoding/binary"
	"io"

	"github.com/cockroachdb/errors"
)

func Read[T any](reader io.ReadSeeker) (result T, err error) {
	return result, binary.Read(reader, binary.LittleEndian, &result)
}

func ReadBytes(reader io.ReadSeeker, size uint64) (bytes []byte, err error) {
	bytes = make([]byte, size)
	if err = binary.Read(reader, binary.LittleEndian, &bytes); err != nil {
		err = errors.Errorf("failed to read %d bytes: %w", size, err)
	}

	return
}

func ReadBlob(reader io.ReadSeeker) (blob []byte, err error) {
	var size uint64
	if size, err = Read[uint64](reader); err != nil {
		err = errors.Errorf("failed to read blob size: %w", err)
	} else if blob, err = ReadBytes(reader, size); err != nil {
		err = errors.Errorf("failed to read blob: %w", err)
	}

	return
}

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
