package marshalutil

import (
	"time"
)

// WriteTime marshals the given time into a sequence of bytes, that get appended to the internal buffer.
func (util *MarshalUtil) WriteTime(timeToWrite time.Time) {
	util.WriteInt64(timeToWrite.UnixNano())
}

// ReadTime unmarshals a time object from the internal read buffer.
func (util *MarshalUtil) ReadTime() (result time.Time, err error) {
	nanoSeconds, err := util.ReadInt64()
	if err != nil {
		return
	}

	result = time.Unix(0, nanoSeconds)

	return
}
