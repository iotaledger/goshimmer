package marshalutil

import (
	"time"
)

// WriteTime marshals the given time into a sequence of bytes, that get appended to the internal buffer.
func (util *MarshalUtil) WriteTime(timeToWrite time.Time) {
	marshaledBytes, err := timeToWrite.MarshalBinary()
	if err != nil {
		// if marshaling fails due to a corrupted time -> we "recover" to the zero value
		marshaledBytes, err = time.Time{}.MarshalBinary()
		if err != nil {
			// if the recovery fails, we panic (sth is really broken)
			panic(err)
		}
	}
	util.WriteBytes(marshaledBytes)
}

// ReadTime unmarshals a time object from the internal read buffer.
func (util *MarshalUtil) ReadTime() (result time.Time, err error) {
	bytes, err := util.ReadBytes(15)
	if err != nil {
		return
	}

	if err = result.UnmarshalBinary(bytes); err != nil {
		util.ReadSeek(util.ReadOffset() - 15)
	}

	return
}
