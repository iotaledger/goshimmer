package stream

import (
	"encoding/binary"
	"io"

	"github.com/cockroachdb/errors"
)

func WriteCollection(writer io.WriteSeeker, writeCollection func() (uint64, error)) (err error) {
	startOffset, seekErr := writer.Seek(8, io.SeekCurrent)
	if seekErr != nil {
		return errors.Errorf("failed to seek to start of attestors: %w", seekErr)
	}

	elementsCount, err := writeCollection()
	if err != nil {
		return errors.Errorf("failed to write collection: %w", err)
	}

	endOffset, seekErr := writer.Seek(0, io.SeekCurrent)
	if seekErr != nil {
		return errors.Errorf("failed to read end offset of attestors: %w", seekErr)
	}

	if _, seekErr = writer.Seek(startOffset-8, io.SeekStart); seekErr != nil {
		return errors.Errorf("failed to seek to start of attestors: %w", seekErr)
	} else if err = binary.Write(writer, binary.LittleEndian, elementsCount); err != nil {
		return errors.Errorf("failed to write attestors count: %w", err)
	} else if _, err = writer.Seek(endOffset, io.SeekStart); err != nil {
		return errors.Errorf("failed to seek to end of attestors: %w", err)
	}

	return nil
}

func ReadCollection(reader io.ReadSeeker, readCallback func(int) error) (err error) {
	var elementsCount uint64
	if err = binary.Read(reader, binary.LittleEndian, &elementsCount); err != nil {
		return errors.Errorf("failed to read attestors count: %w", err)
	}

	for i := 0; i < int(elementsCount); i++ {
		if err = readCallback(i); err != nil {
			return errors.Errorf("failed to read element %d: %w", i, err)
		}
	}
	
	return nil
}
