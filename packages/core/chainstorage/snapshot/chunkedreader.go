package snapshot

import (
	"encoding/binary"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/options"
)

const defaultChunkSize = 100

type ChunkedReader[A any, B constraints.MarshalablePtr[A]] struct {
	fileHandle      *os.File
	consumedCounter uint32

	objCount  uint32
	objSize   uint32
	chunkSize int
}

func NewChunkedReader[A any, B constraints.MarshalablePtr[A]](fileHandle *os.File, opts ...options.Option[ChunkedReader[A, B]]) (new *ChunkedReader[A, B]) {
	return options.Apply(&ChunkedReader[A, B]{
		fileHandle: fileHandle,
		chunkSize:  defaultChunkSize,
	},
		opts,
		(*ChunkedReader[A, B]).readObjCount,
		(*ChunkedReader[A, B]).readObjSize)
}

func (c *ChunkedReader[A, B]) readObjSize() {
	if err := binary.Read(c.fileHandle, binary.LittleEndian, &c.objSize); err != nil {
		panic(err)
	}
}

func (c *ChunkedReader[A, B]) readObjCount() {
	if err := binary.Read(c.fileHandle, binary.LittleEndian, &c.objCount); err != nil {
		panic(err)
	}
}

func (c *ChunkedReader[A, B]) ReadChunk() (chunk []B, err error) {
	chunk = make([]B, 0)

	if c.IsFinished() {
		return nil, errors.Errorf("tried to read more than the declared objects")
	}

	for chunckPos := 0; chunckPos < c.chunkSize && c.consumedCounter < c.objCount; chunckPos++ {
		objBytes := make([]byte, c.objSize)
		if err = binary.Read(c.fileHandle, binary.LittleEndian, objBytes); err != nil {
			return
		}

		obj := B(new(A))
		if consumedBytes, err := obj.FromBytes(objBytes); err != nil || consumedBytes != len(objBytes) {
			return nil, errors.Errorf("failed to deserialize object: %w", err)
		}

		chunk = append(chunk, obj)
		c.consumedCounter++
	}

	return
}

func (c *ChunkedReader[A, B]) IsFinished() bool {
	return c.consumedCounter == c.objCount
}

func (c *ChunkedReader[A, B]) ConsumedCounter() uint32 {
	return c.consumedCounter
}

func (c *ChunkedReader[A, B]) ObjSize() uint32 {
	return c.objSize
}

func (c *ChunkedReader[A, B]) ChunkSize() int {
	return c.chunkSize
}

func WithChunkSize[A any, B constraints.MarshalablePtr[A]](chunkSize int) options.Option[ChunkedReader[A, B]] {
	return func(chunkReader *ChunkedReader[A, B]) {
		chunkReader.chunkSize = chunkSize
	}
}
