package byteutils

func ReadAvailableBytesToBuffer(target []byte, targetOffset int, source []byte, sourceOffset int, sourceLength int) int {
	availableBytes := sourceLength - sourceOffset
	requiredBytes := len(target) - targetOffset

	var bytesToRead int
	if availableBytes < requiredBytes {
		bytesToRead = availableBytes
	} else {
		bytesToRead = requiredBytes
	}

	copy(target[targetOffset:], source[sourceOffset:sourceOffset+bytesToRead])

	return bytesToRead
}
