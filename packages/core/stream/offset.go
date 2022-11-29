package stream

import (
	"io"
)

func Offset(seeker io.Seeker) (offset int64, err error) {
	return seeker.Seek(0, io.SeekCurrent)
}

func Skip(seeker io.Seeker, offset int64) (newOffset int64, err error) {
	return seeker.Seek(offset, io.SeekCurrent)
}

func GoTo(seeker io.Seeker, offset int64) (newOffset int64, err error) {
	return seeker.Seek(offset, io.SeekStart)
}
