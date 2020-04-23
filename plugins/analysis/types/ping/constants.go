package ping

const (
	// MarshaledPacketHeader defines the MarshaledPacketHeader.
	MarshaledPacketHeader = 0x00

	// MarshaledPacketHeaderStart defines the start of the MarshaledPacketHeader.
	MarshaledPacketHeaderStart = 0
	// MarshaledPacketHeaderSize defines the size of the MarshaledPacketHeader.
	MarshaledPacketHeaderSize = 1
	// MarshaledPacketHeaderEnd defines the end of the MarshaledPacketHeader.
	MarshaledPacketHeaderEnd = MarshaledPacketHeaderStart + MarshaledPacketHeaderSize

	// MarshaledTotalSize defines the total size of the MarshaledPacketHeader.
	MarshaledTotalSize = MarshaledPacketHeaderEnd
)
