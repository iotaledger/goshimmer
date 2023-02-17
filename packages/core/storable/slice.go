package storable

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

const SliceOffsetAuto = ^int(0)

type Slice[A any, B constraints.MarshalablePtr[A]] struct {
	fileHandle  *os.File
	startOffset int
	entrySize   int

	sync.RWMutex
}

func NewSlice[A any, B constraints.MarshalablePtr[A]](fileName string, entrySize int, opts ...options.Option[Slice[A, B]]) (indexedFile *Slice[A, B], err error) {
	return options.Apply(new(Slice[A, B]), opts, func(i *Slice[A, B]) {
		if i.fileHandle, err = os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0o666); err != nil {
			err = errors.Wrap(err, "failed to open file")
			return
		}

		i.entrySize = entrySize

		if err = i.readHeader(); err != nil {
			err = errors.Wrap(err, "failed to read header")
			return
		}
	}), err
}

func (i *Slice[A, B]) Set(index int, entry B) (err error) {
	i.Lock()
	defer i.Unlock()

	serializedEntry, err := entry.Bytes()
	if err != nil {
		return errors.Wrap(err, "failed to serialize entry")
	}

	if i.startOffset == SliceOffsetAuto {
		i.startOffset = index

		if err = i.writeHeader(); err != nil {
			return errors.Wrap(err, "failed to write header")
		}
	}

	relativeIndex := index - i.startOffset
	if relativeIndex < 0 {
		return errors.Errorf("index %d is out of bounds", index)
	}

	if _, err = i.fileHandle.WriteAt(serializedEntry, int64(8+relativeIndex*i.entrySize)); err != nil {
		return errors.Wrap(err, "failed to write entry")
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
		return entry, errors.Wrap(err, "failed to read entry")
	}

	var newEntry B = new(A)
	if _, err = newEntry.FromBytes(entryBytes); err != nil {
		return entry, errors.Wrap(err, "failed to deserialize entry")
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

		return errors.Wrap(err, "failed to read start offset")
	}

	var startOffset uint64
	if _, err = serix.DefaultAPI.Decode(context.Background(), startOffsetBytes, &startOffset); err != nil {
		return errors.Wrap(err, "failed to decode start offset")
	}

	if i.startOffset != 0 && i.startOffset != SliceOffsetAuto {
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
		return errors.Wrap(err, "failed to encode startOffset")
	}

	entrySizeBytes, err := serix.DefaultAPI.Encode(context.Background(), uint64(i.entrySize))
	if err != nil {
		return errors.Wrap(err, "failed to encode entrySize")
	}

	if _, err = i.fileHandle.WriteAt(startOffsetBytes, 0); err != nil {
		return errors.Wrap(err, "failed to write startOffset")
	} else if _, err = i.fileHandle.WriteAt(entrySizeBytes, 8); err != nil {
		return errors.Wrap(err, "failed to write entrySize")
	}

	return i.fileHandle.Sync()
}

func WithOffset[A any, B constraints.MarshalablePtr[A]](offset int) options.Option[Slice[A, B]] {
	return func(s *Slice[A, B]) {
		s.startOffset = offset
	}
}
