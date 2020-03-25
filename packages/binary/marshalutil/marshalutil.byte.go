package marshalutil

func (util *MarshalUtil) WriteByte(byte byte) {
	writeEndOffset := util.expandWriteCapacity(1)

	util.bytes[util.writeOffset] = byte

	util.WriteSeek(writeEndOffset)
}

func (util *MarshalUtil) ReadByte() (byte, error) {
	readEndOffset, err := util.checkReadCapacity(1)
	if err != nil {
		return 0, err
	}

	defer util.ReadSeek(readEndOffset)

	return util.bytes[util.readOffset], nil
}
