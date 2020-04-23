package heartbeat

import "crypto/sha256"

const (
	// MarshaledPacketHeader unique identifier of packet.
	MarshaledPacketHeader = 0x01

	// MaxOutboundNeighborCount is the maximum number of allowed neighbors in one direction.
	MaxOutboundNeighborCount = 4
	// MaxInboundNeighborCount is the maximum number of allowed neighbors in one direction.
	MaxInboundNeighborCount = 4

	// MaxMarshaledTotalSize Maximum packet length in bytes.
	MaxMarshaledTotalSize = MarshaledPacketHeaderSize + MarshaledOwnIDSize +
		MarshaledOutboundIDsLengthSize + MaxOutboundNeighborCount*MarshaledOutboundIDSize +
		MarshaledInboundIDsLengthSize + MaxInboundNeighborCount*MarshaledInboundIDSize

	// MarshaledPacketHeaderStart is the beginning of the MarshaledPacketHeader.
	MarshaledPacketHeaderStart = 0
	// MarshaledPacketHeaderSize is the size of the MarshaledPacketHeader.
	MarshaledPacketHeaderSize = 1
	// MarshaledPacketHeaderEnd is the end of the MarshaledPacketHeader.
	MarshaledPacketHeaderEnd = MarshaledPacketHeaderStart + MarshaledPacketHeaderSize

	// MarshaledOwnIDStart is the beginning of the MarshaledOwnID.
	MarshaledOwnIDStart = MarshaledPacketHeaderEnd
	// MarshaledOwnIDSize is the size of the MarshaledOwnID.
	MarshaledOwnIDSize = sha256.Size
	// MarshaledOwnIDEnd is the end of the MarshaledOwnID.
	MarshaledOwnIDEnd = MarshaledOwnIDStart + MarshaledOwnIDSize

	// MarshaledOutboundIDsLengthStart is the beginning of the MarshaledOutboundIDsLength.
	MarshaledOutboundIDsLengthStart = MarshaledOwnIDEnd
	// MarshaledOutboundIDsLengthSize is the size of the MarshaledOutboundIDsLength.
	MarshaledOutboundIDsLengthSize = 1
	// MarshaledOutboundIDSize is the size of the MarshaledOutboundID.
	MarshaledOutboundIDSize = sha256.Size
	// MarshaledOutboundIDsLengthEnd is the end of the MarshaledOutboundIDsLength.
	MarshaledOutboundIDsLengthEnd = MarshaledOutboundIDsLengthStart + MarshaledOutboundIDsLengthSize

	// MarshaledInboundIDsLengthSize is the size of the MarshaledInboundIDsLength.
	MarshaledInboundIDsLengthSize = 1
	// MarshaledInboundIDSize is the size of the MarshaledInboundID.
	MarshaledInboundIDSize = sha256.Size
)
