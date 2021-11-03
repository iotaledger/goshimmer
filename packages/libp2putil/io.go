package libp2putil

import (
	"bufio"
	"io"

	"github.com/multiformats/go-varint"
	"google.golang.org/protobuf/proto"
)

type UvarintWriter struct {
	w      io.Writer
	lenBuf []byte
}

func NewDelimitedWriter(w io.Writer) *UvarintWriter {
	return &UvarintWriter{w, make([]byte, varint.MaxLenUvarint63)}
}

func (uw *UvarintWriter) WriteMsg(msg proto.Message) (err error) {
	var data []byte
	data, err = proto.Marshal(msg)
	if err != nil {
		return err
	}
	length := uint64(len(data))
	n := varint.PutUvarint(uw.lenBuf, length)
	_, err = uw.w.Write(uw.lenBuf[:n])
	if err != nil {
		return err
	}
	_, err = uw.w.Write(data)
	return err
}

type UvarintReader struct {
	r   *bufio.Reader
	buf []byte
}

func NewDelimitedReader(r io.Reader) *UvarintReader {
	return &UvarintReader{r: bufio.NewReader(r)}
}

func (ur *UvarintReader) ReadMsg(msg proto.Message) error {
	length64, err := varint.ReadUvarint(ur.r)
	if err != nil {
		return err
	}
	length := int(length64)
	if len(ur.buf) < length {
		ur.buf = make([]byte, length)
	}
	buf := ur.buf[:length]
	if _, err := io.ReadFull(ur.r, buf); err != nil {
		return err
	}
	return proto.Unmarshal(buf, msg)
}
