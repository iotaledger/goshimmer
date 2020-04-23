package heartbeat

import "crypto/sha256"

const (
	// MarshaledPacketHeader unique identifier of packet
	MarshaledPacketHeader = 0x01

	// MaxOutboundNeighborCount is the maximum number of allowed neighbors in one direction
	MaxOutboundNeighborCount = 4
	// MaxInboundNeighborCount is the maximum number of allowed neighbors in one direction
	MaxInboundNeighborCount  = 4

	// MaxMarshaledTotalSize Maximum packet length in bytes
	MaxMarshaledTotalSize = MarshaledPacketHeaderSize + MarshaledOwnIDSize +
		MarshaledOutboundIDsLengthSize + MaxOutboundNeighborCount*MarshaledOutboundIDSize +
		MarshaledInboundIDsLengthSize + MaxInboundNeighborCount*MarshaledInboundIDSize

	MarshaledPacketHeaderStart = 0
	MarshaledPacketHeaderSize  = 1
	MarshaledPacketHeaderEnd   = MarshaledPacketHeaderStart + MarshaledPacketHeaderSize

	MarshaledOwnIDStart = MarshaledPacketHeaderEnd
	MarshaledOwnIDSize  = sha256.Size
	MarshaledOwnIDEnd   = MarshaledOwnIDStart + MarshaledOwnIDSize

	MarshaledOutboundIDsLengthStart = MarshaledOwnIDEnd
	MarshaledOutboundIDsLengthSize  = 1
	MarshaledOutboundIDSize         = sha256.Size
	MarshaledOutboundIDsLengthEnd   = MarshaledOutboundIDsLengthStart + MarshaledOutboundIDsLengthSize

	MarshaledInboundIDsLengthSize = 1
	MarshaledInboundIDSize        = sha256.Size
)
