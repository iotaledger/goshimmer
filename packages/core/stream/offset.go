package stream

import (
	"io"
)

// Offset returns the current offset of the reader.
func Offset(seeker io.Seeker) (offset int64, err error) {
	return seeker.Seek(0, io.SeekCurrent)
}

// Skip skips the given number of bytes and returns the new offset.
func Skip(seeker io.Seeker, offset int64) (newOffset int64, err error) {
	return seeker.Seek(offset, io.SeekCurrent)
}

// GoTo seeks to the given offset and returns the new offset.
func GoTo(seeker io.Seeker, offset int64) (newOffset int64, err error) {
	return seeker.Seek(offset, io.SeekStart)
}
