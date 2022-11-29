package stream

import (
	"encoding/binary"
	"io"

	"github.com/cockroachdb/errors"
)

func Write[T any](writer io.WriteSeeker, value T) (err error) {
	return binary.Write(writer, binary.LittleEndian, value)
}

func WriteBlob(writer io.WriteSeeker, blob []byte) (err error) {
	if err = Write(writer, uint64(len(blob))); err != nil {
		err = errors.Errorf("failed to write blob length: %w", err)
	} else if err = Write(writer, blob); err != nil {
		err = errors.Errorf("failed to write blob: %w", err)
	}

	return
}

func WriteCollection(writer io.WriteSeeker, writeCollection func() (elementsCount uint64, err error)) (err error) {
	var elementsCount uint64
	var startOffset, endOffset int64
	if startOffset, err = Offset(writer); err != nil {
		err = errors.Errorf("failed to get start offset: %w", err)
	} else if _, err = Skip(writer, 8); err != nil {
		err = errors.Errorf("failed to skip elements count: %w", err)
	} else if elementsCount, err = writeCollection(); err != nil {
		err = errors.Errorf("failed to write collection: %w", err)
	} else if endOffset, err = Offset(writer); err != nil {
		err = errors.Errorf("failed to read end offset of collection: %w", err)
	} else if _, err = GoTo(writer, startOffset); err != nil {
		err = errors.Errorf("failed to seek to start of attestors: %w", err)
	} else if err = Write(writer, elementsCount); err != nil {
		err = errors.Errorf("failed to write attestors count: %w", err)
	} else if _, err = GoTo(writer, endOffset); err != nil {
		err = errors.Errorf("failed to seek to end of attestors: %w", err)
	}

	return
}
