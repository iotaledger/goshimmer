package marshalutil

import "encoding/binary"

const UINT64_SIZE = 8

func (util *MarshalUtil) WriteUint64(value uint64) {
	writeEndOffset := util.expandWriteCapacity(UINT64_SIZE)

	binary.LittleEndian.PutUint64(util.bytes[util.writeOffset:writeEndOffset], value)

	util.WriteSeek(writeEndOffset)
}

func (util *MarshalUtil) ReadUint64() (uint64, error) {
	readEndOffset, err := util.checkReadCapacity(UINT64_SIZE)
	if err != nil {
		return 0, err
	}

	defer util.ReadSeek(readEndOffset)

	return binary.LittleEndian.Uint64(util.bytes[util.readOffset:readEndOffset]), nil
}
