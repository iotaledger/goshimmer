package marshalutil

import (
	"time"
)

// WriteTime marshals the given time into a sequence of bytes, that get appended to the internal buffer.
func (util *MarshalUtil) WriteTime(timeToWrite time.Time) {
	nanoSeconds := timeToWrite.UnixNano()

	// the zero value of time translates to -6795364578871345152
	if nanoSeconds == -6795364578871345152 {
		util.WriteInt64(0)
	} else {
		util.WriteInt64(timeToWrite.UnixNano())
	}
}

// ReadTime unmarshals a time object from the internal read buffer.
func (util *MarshalUtil) ReadTime() (result time.Time, err error) {
	nanoSeconds, err := util.ReadInt64()
	if err != nil {
		return
	}

	if nanoSeconds == 0 {
		result = time.Time{}
	} else {
		result = time.Unix(0, nanoSeconds)
	}

	return
}
