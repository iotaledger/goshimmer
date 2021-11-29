package libp2putil

import (
	"bufio"
	"fmt"
	"io"

	"github.com/multiformats/go-varint"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

// UvarintWriter writes protobuf messages.
type UvarintWriter struct {
	w io.Writer
}

// NewDelimitedWriter returns a new UvarintWriter.
func NewDelimitedWriter(w io.Writer) *UvarintWriter {
	return &UvarintWriter{w}
}

// WriteMsg writes protobuf message.
func (uw *UvarintWriter) WriteMsg(msg proto.Message) (err error) {
	var data []byte
	lenBuf := make([]byte, varint.MaxLenUvarint63)
	data, err = proto.Marshal(msg)
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

// UvarintReader read protobuf messages.
type UvarintReader struct {
	r *bufio.Reader
}

// NewDelimitedReader returns a new UvarintReader.
func NewDelimitedReader(r io.Reader) *UvarintReader {
	return &UvarintReader{r: bufio.NewReader(r)}
}

// ReadMsg read protobuf messages.
func (ur *UvarintReader) ReadMsg(msg proto.Message) error {
	length64, err := varint.ReadUvarint(ur.r)
	if err != nil {
		return err
	}
	if length64 > tangle.MaxMessageSize {
		return fmt.Errorf("max message size exceeded: %d", length64)
	}
	buf := make([]byte, length64)
	if _, err := io.ReadFull(ur.r, buf); err != nil {
		return err
	}
	return proto.Unmarshal(buf, msg)
}
