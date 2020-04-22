package ping

const (
	MarshaledPacketHeader = 0x00

	MarshaledPacketHeaderStart = 0
	MarshaledPacketHeaderSize  = 1
	MarshaledPacketHeaderEnd   = MarshaledPacketHeaderStart + MarshaledPacketHeaderSize

	MarshaledTotalSize = MarshaledPacketHeaderEnd
)
