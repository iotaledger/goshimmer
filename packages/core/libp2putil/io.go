package libp2putil

import (
	"bufio"
	"io"

	"github.com/multiformats/go-varint"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// UvarintWriter writes protobuf blocks.
type UvarintWriter struct {
	w io.Writer
}

// NewDelimitedWriter returns a new UvarintWriter.
func NewDelimitedWriter(w io.Writer) *UvarintWriter {
	return &UvarintWriter{w}
}

// WriteBlk writes protobuf block.
func (uw *UvarintWriter) WriteBlk(blk proto.Message) (err error) {
	var data []byte
	lenBuf := make([]byte, varint.MaxLenUvarint63)
	data, err = proto.Marshal(blk)
	if err != nil {
		return err
	}
	length := uint64(len(data))
	n := varint.PutUvarint(lenBuf, length)
	_, err = uw.w.Write(lenBuf[:n])
	if err != nil {
		return err
	}
	_, err = uw.w.Write(data)
	return err
}

// UvarintReader read protobuf blocks.
type UvarintReader struct {
	r *bufio.Reader
}

// NewDelimitedReader returns a new UvarintReader.
func NewDelimitedReader(r io.Reader) *UvarintReader {
	return &UvarintReader{r: bufio.NewReader(r)}
}

// ReadBlk read protobuf blocks.
func (ur *UvarintReader) ReadBlk(blk proto.Message) error {
	length64, err := varint.ReadUvarint(ur.r)
	if err != nil {
		return err
	}
	if length64 > models.MaxBlockSize {
		return errors.Errorf("max block size exceeded: %d", length64)
	}
	buf := make([]byte, length64)
	if _, err := io.ReadFull(ur.r, buf); err != nil {
		return err
	}
	return proto.Unmarshal(buf, blk)
}
