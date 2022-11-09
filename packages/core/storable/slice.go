package storable

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/serix"
)

const SliceOffsetAuto = ^int(0)

type Slice[A any, B constraints.MarshalablePtr[A]] struct {
	fileHandle  *os.File
	startOffset int
	entrySize   int

	sync.RWMutex
}

func NewSlice[A any, B constraints.MarshalablePtr[A]](fileName string, opts ...options.Option[Slice[A, B]]) (indexedFile *Slice[A, B], err error) {
	return options.Apply(new(Slice[A, B]), opts, func(i *Slice[A, B]) {
		if i.fileHandle, err = os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0o666); err != nil {
			err = errors.Errorf("failed to open file: %w", err)
			return
		}

		serializedEntity, serializationErr := B(new(A)).Bytes()
		if serializationErr != nil {
			err = errors.Errorf("failed to serialize empty entity (to determine its length): %w", serializationErr)
			return
		}
		i.entrySize = len(serializedEntity)

		if err = i.readHeader(); err != nil {
			err = errors.Errorf("failed to read header: %w", err)
			return
		}
	}), err
}

func (i *Slice[A, B]) Set(index int, entry B) (err error) {
	i.Lock()
	defer i.Unlock()

	serializedEntry, err := entry.Bytes()
	if err != nil {
		return errors.Errorf("failed to serialize entry: %w", err)
	}

	if i.startOffset == int(SliceOffsetAuto) {
		i.startOffset = index

		if err = i.writeHeader(); err != nil {
			return errors.Errorf("failed to write header: %w", err)
		}
	}

	relativeIndex := index - i.startOffset
	if relativeIndex < 0 {
		return errors.Errorf("index %d is out of bounds", index)
	}

	if _, err = i.fileHandle.WriteAt(serializedEntry, int64(8+relativeIndex*i.entrySize)); err != nil {
		return errors.Errorf("failed to write entry: %w", err)
	}

	return i.fileHandle.Sync()
}

func (i *Slice[A, B]) Get(index int) (entry B, err error) {
	relativeIndex := index - i.startOffset
	if relativeIndex < 0 {
		return entry, errors.Errorf("index %d is out of bounds", index)
	}

	entryBytes := make([]byte, i.entrySize)
	if _, err = i.fileHandle.ReadAt(entryBytes, int64(8+relativeIndex*i.entrySize)); err != nil {
		return entry, errors.Errorf("failed to read entry: %w", err)
	}

	var newEntry B = new(A)
	if _, err = newEntry.FromBytes(entryBytes); err != nil {
		return entry, errors.Errorf("failed to deserialize entry: %w", err)
	}
	entry = newEntry

	return
}

func (i *Slice[A, B]) Close() (err error) {
	return i.fileHandle.Close()
}

func (i *Slice[A, B]) readHeader() (err error) {
	startOffsetBytes := make([]byte, 8)
	if _, err = i.fileHandle.ReadAt(startOffsetBytes, 0); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}

		return errors.Errorf("failed to read start offset: %w", err)
	}

	var startOffset uint64
	if _, err = serix.DefaultAPI.Decode(context.Background(), startOffsetBytes, &startOffset); err != nil {
		return errors.Errorf("failed to decode start offset: %w", err)
	}

	if i.startOffset != 0 && i.startOffset != int(SliceOffsetAuto) {
		if int(startOffset) != i.startOffset {
			return errors.Errorf("start offset %d does not match existing offset %d in file", i.startOffset, startOffset)
		}
	}

	i.startOffset = int(startOffset)

	return nil
}

func (i *Slice[A, B]) writeHeader() (err error) {
	startOffsetBytes, err := serix.DefaultAPI.Encode(context.Background(), uint64(i.startOffset))
	if err != nil {
		return errors.Errorf("failed to encode startOffset: %w", err)
	}

	entrySizeBytes, err := serix.DefaultAPI.Encode(context.Background(), uint64(i.entrySize))
	if err != nil {
		return errors.Errorf("failed to encode entrySize: %w", err)
	}

	if _, err = i.fileHandle.WriteAt(startOffsetBytes, 0); err != nil {
		return errors.Errorf("failed to write startOffset: %w", err)
	} else if _, err = i.fileHandle.WriteAt(entrySizeBytes, 8); err != nil {
		return errors.Errorf("failed to write entrySize: %w", err)
	}

	return i.fileHandle.Sync()
}

func WithOffset[A any, B constraints.MarshalablePtr[A]](offset int) options.Option[Slice[A, B]] {
	return func(s *Slice[A, B]) {
		s.startOffset = offset
	}
}
