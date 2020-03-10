package marshalutil

import (
	"time"
)

func (util *MarshalUtil) WriteTime(timeToWrite time.Time) {
	marshaledBytes, err := timeToWrite.MarshalBinary()
	if err != nil {
		marshaledBytes, _ = time.Time{}.MarshalBinary()
	}
	util.WriteBytes(marshaledBytes)
}

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
