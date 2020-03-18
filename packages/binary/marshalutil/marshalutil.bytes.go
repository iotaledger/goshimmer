package marshalutil

// WriteBytes appends the given bytes to the internal buffer.
// It returns the same MarshalUtil so calls can be chained.
func (util *MarshalUtil) WriteBytes(bytes []byte) *MarshalUtil {
	writeEndOffset := util.expandWriteCapacity(len(bytes))

	copy(util.bytes[util.writeOffset:writeEndOffset], bytes)

	util.WriteSeek(writeEndOffset)

	return util
}

// ReadBytes unmarshals the given amount of bytes from the internal read buffer.
func (util *MarshalUtil) ReadBytes(length int) ([]byte, error) {
	if length < 0 {
		length = len(util.bytes) - util.readOffset + length
	}

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
