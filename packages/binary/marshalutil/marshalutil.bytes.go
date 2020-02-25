package marshalutil

func (util *MarshalUtil) WriteBytes(bytes []byte) {
	writeEndOffset := util.expandWriteCapacity(len(bytes))

	copy(util.bytes[util.writeOffset:writeEndOffset], bytes)

	util.WriteSeek(writeEndOffset)
}

func (util *MarshalUtil) ReadBytes(length int) ([]byte, error) {
	readEndOffset, err := util.checkReadCapacity(length)
	if err != nil {
		return nil, err
	}

	defer util.ReadSeek(readEndOffset)

	return util.bytes[util.readOffset:readEndOffset], nil
}

func (util *MarshalUtil) ReadRemainingBytes() []byte {
	defer util.ReadSeek(util.size)

	return util.bytes[util.readOffset:]
}
