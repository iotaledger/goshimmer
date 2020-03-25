package marshalutil

import (
	"encoding/binary"
)

const INT64_SIZE = 8

func (util *MarshalUtil) WriteInt64(value int64) {
	writeEndOffset := util.expandWriteCapacity(INT64_SIZE)

	binary.LittleEndian.PutUint64(util.bytes[util.writeOffset:writeEndOffset], uint64(value))

	util.WriteSeek(writeEndOffset)
}

func (util *MarshalUtil) ReadInt64() (int64, error) {
	readEndOffset, err := util.checkReadCapacity(INT64_SIZE)
	if err != nil {
		return 0, err
	}

	defer util.ReadSeek(readEndOffset)

	return int64(binary.LittleEndian.Uint64(util.bytes[util.readOffset:readEndOffset])), nil
}
