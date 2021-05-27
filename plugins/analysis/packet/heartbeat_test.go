package packet_test

import (
	"crypto/sha256"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/protocol/tlv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/iotaledger/goshimmer/plugins/analysis/packet"
)

var (
	ownID     = sha256.Sum256([]byte{'A'})
	networkID = []byte("v0.2.0")
)

func TestNewHeartbeatMessage(t *testing.T) {
	testCases := []struct {
		hb  *Heartbeat
		err error
	}{
		// ok, packet with max inbound/outbound peer IDs
		{
			hb: func() *Heartbeat {
				outboundIDs := make([][]byte, HeartbeatMaxOutboundPeersCount)
				inboundIDs := make([][]byte, HeartbeatMaxInboundPeersCount)
				for i := 0; i < HeartbeatMaxOutboundPeersCount; i++ {
					outboundID := sha256.Sum256([]byte{byte(i)})
					inboundID := sha256.Sum256([]byte{byte(i + HeartbeatMaxOutboundPeersCount)})
					outboundIDs[i] = outboundID[:]
					inboundIDs[i] = inboundID[:]
				}
				return &Heartbeat{NetworkID: networkID, OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			}(),
			err: nil,
		},
		// ok, packet with only inbound peer IDs
		{
			hb: func() *Heartbeat {
				outboundIDs := make([][]byte, 0)
				inboundIDs := make([][]byte, HeartbeatMaxInboundPeersCount)
				for i := 0; i < HeartbeatMaxInboundPeersCount; i++ {
					inboundID := sha256.Sum256([]byte{byte(i)})
					inboundIDs[i] = inboundID[:]
				}
				return &Heartbeat{NetworkID: networkID, OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			}(),
			err: nil,
		},
		// ok, packet with no peer IDs (excluding own ID) is legit too
		{
			hb: func() *Heartbeat {
				outboundIDs := make([][]byte, 0)
				inboundIDs := make([][]byte, 0)
				return &Heartbeat{NetworkID: networkID, OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			}(),
			err: nil,
		},
		// err, has no own peer ID
		{
			hb: func() *Heartbeat {
				return &Heartbeat{NetworkID: networkID, OwnID: nil, OutboundIDs: make([][]byte, 0), InboundIDs: make([][]byte, 0)}
			}(),
			err: ErrInvalidHeartbeat,
		},
		// err, outbound ID count exceeds maximum
		{
			hb: func() *Heartbeat {
				outboundIDs := make([][]byte, 5)
				for i := 0; i < len(outboundIDs); i++ {
					outboundID := sha256.Sum256([]byte{byte(i)})
					outboundIDs[i] = outboundID[:]
				}
				return &Heartbeat{NetworkID: networkID, OwnID: ownID[:], OutboundIDs: outboundIDs}
			}(),
			err: ErrInvalidHeartbeat,
		},
		// err, inbound ID count exceeds maximum
		{
			hb: func() *Heartbeat {
				outboundIDs := make([][]byte, 0)
				inboundIDs := make([][]byte, 5)
				for i := 0; i < len(inboundIDs); i++ {
					inboundID := sha256.Sum256([]byte{byte(i + 10)})
					inboundIDs[i] = inboundID[:]
				}
				return &Heartbeat{NetworkID: networkID, OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			}(),
			err: ErrInvalidHeartbeat,
		},
		// err, networkID bytes count exceeds maximum
		{
			hb: func() *Heartbeat {
				outboundIDs := make([][]byte, 0)
				inboundIDs := make([][]byte, 0)
				return &Heartbeat{NetworkID: []byte("v.999.999.99"), OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			}(),
			err: ErrInvalidHeartbeat,
		},
		// err, networkID is zero
		{
			hb: func() *Heartbeat {
				outboundIDs := make([][]byte, 0)
				inboundIDs := make([][]byte, 0)
				return &Heartbeat{NetworkID: []byte(""), OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			}(),
			err: ErrInvalidHeartbeat,
		},
	}
	for _, testCase := range testCases {
		hb := testCase.hb
		serializedHb, err := NewHeartbeatMessage(hb)
		tlvHeaderLength := int(tlv.HeaderMessageDefinition.MaxBytesLength)
		if testCase.err != nil {
			require.True(t, errors.Is(err, testCase.err))
			continue
		}

		require.NoError(t, err, "heartbeat should have been serialized successfully")
		assert.EqualValues(t, MessageTypeHeartbeat, serializedHb[0], "expected heartbeat message type tlv header value as first byte")
		assert.EqualValues(t, len(networkID), int(serializedHb[tlvHeaderLength]), "expected network id to have length %d", len(networkID))
		ownIDOffset := tlvHeaderLength + HeartbeatPacketNetworkIDBytesCountSize + len(networkID)
		assert.EqualValues(t, hb.OwnID, serializedHb[ownIDOffset:ownIDOffset+HeartbeatPacketPeerIDSize], "expected own peer id to be within range of %d:%d", ownIDOffset, ownIDOffset+HeartbeatPacketPeerIDSize)
		assert.EqualValues(t, len(hb.OutboundIDs), serializedHb[ownIDOffset+HeartbeatPacketPeerIDSize], "expected outbound IDs count of %d", len(hb.OutboundIDs))

		// after the outbound IDs count, the outbound IDs are serialized
		offset := tlvHeaderLength + HeartbeatPacketMinSize + len(networkID)
		for i := 0; i < len(hb.OutboundIDs); i++ {
			assert.EqualValues(t, hb.OutboundIDs[i], serializedHb[offset+i*HeartbeatPacketPeerIDSize:offset+(i+1)*HeartbeatPacketPeerIDSize], "outbound ID at the given position doesn't match")
		}

		// shift to offset after outbound IDs
		offset += len(hb.OutboundIDs) * HeartbeatPacketPeerIDSize
		for i := 0; i < len(hb.InboundIDs); i++ {
			assert.EqualValues(t, hb.InboundIDs[i], serializedHb[offset+i*HeartbeatPacketPeerIDSize:offset+(i+1)*HeartbeatPacketPeerIDSize], "inbound ID at the given position doesn't match")
		}
	}
}

func TestParseHeartbeat(t *testing.T) {
	tlvHeaderLength := int(tlv.HeaderMessageDefinition.MaxBytesLength)
	type testcase struct {
		index    int
		source   []byte
		expected *Heartbeat
		err      error
	}
	testCases := []testcase{
		// ok
		func() testcase {
			hb := &Heartbeat{NetworkID: networkID, OwnID: ownID[:], OutboundIDs: make([][]byte, 0), InboundIDs: make([][]byte, 0)}
			// message = tlv header + packet
			// ParseHeartbeat() expects only the packet, hence serializedHb[tlvHeaderLength:]
			serializedHb, _ := NewHeartbeatMessage(hb)
			return testcase{index: 0, source: serializedHb[tlvHeaderLength:], expected: hb, err: nil}
		}(),
		// ok
		func() testcase {
			outboundIDs := make([][]byte, HeartbeatMaxOutboundPeersCount)
			inboundIDs := make([][]byte, HeartbeatMaxInboundPeersCount)
			for i := 0; i < HeartbeatMaxOutboundPeersCount; i++ {
				outboundID := sha256.Sum256([]byte{byte(i)})
				inboundID := sha256.Sum256([]byte{byte(i + HeartbeatMaxOutboundPeersCount)})
				outboundIDs[i] = outboundID[:]
				inboundIDs[i] = inboundID[:]
			}
			hb := &Heartbeat{NetworkID: networkID, OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			// message = tlv header + packet
			// ParseHeartbeat() expects only the packet, hence serializedHb[tlvHeaderLength:]
			serializedHb, _ := NewHeartbeatMessage(hb)
			return testcase{index: 1, source: serializedHb[tlvHeaderLength:], expected: hb, err: nil}
		}(),
		// err, exceeds max inbound peer IDs
		func() testcase {
			// this lets us add one more inbound peer ID at the end
			outboundIDs := make([][]byte, HeartbeatMaxOutboundPeersCount-1)
			inboundIDs := make([][]byte, HeartbeatMaxInboundPeersCount)
			for i := 0; i < HeartbeatMaxInboundPeersCount; i++ {
				inboundID := sha256.Sum256([]byte{byte(i + HeartbeatMaxOutboundPeersCount)})
				if i != HeartbeatMaxInboundPeersCount-1 {
					outboundID := sha256.Sum256([]byte{byte(i)})
					outboundIDs[i] = outboundID[:]
				}
				inboundIDs[i] = inboundID[:]
			}
			hb := &Heartbeat{NetworkID: networkID, OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			// message = tlv header + packet
			// ParseHeartbeat() expects only the packet, hence serializedHb[tlvHeaderLength:]
			serializedHb, _ := NewHeartbeatMessage(hb)
			// add an additional peer id
			serializedHb = append(serializedHb, ownID[:]...)
			return testcase{index: 2, source: serializedHb[tlvHeaderLength:], expected: hb, err: ErrMalformedPacket}
		}(),
		// err, exceeds max outbound peer IDs
		func() testcase {
			outboundIDs := make([][]byte, HeartbeatMaxOutboundPeersCount)
			for i := 0; i < HeartbeatMaxInboundPeersCount; i++ {
				outboundID := sha256.Sum256([]byte{byte(i)})
				outboundIDs[i] = outboundID[:]
			}
			hb := &Heartbeat{NetworkID: networkID, OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: make([][]byte, 0)}
			// NewHeartbeatMessage would return nil and and error for a malformed packet (too many outbound peer IDs)
			// so we create a correct message(tlv header + packet)
			serializedHb, _ := NewHeartbeatMessage(hb)
			// and add an extra outbound ID (inbound IDs are zero)
			serializedHb = append(serializedHb, ownID[:]...)
			// manually overwrite outboundIDCount
			serializedHb[tlvHeaderLength+HeartbeatPacketMinSize+len(networkID)-1] = HeartbeatMaxOutboundPeersCount + 1
			return testcase{index: 3, source: serializedHb[tlvHeaderLength:], expected: hb, err: ErrMalformedPacket}
		}(),
		// err, advertised outbound ID count is bigger than remaining data
		func() testcase {
			outboundIDs := make([][]byte, HeartbeatMaxOutboundPeersCount-1)
			for i := 0; i < HeartbeatMaxOutboundPeersCount-1; i++ {
				outboundID := sha256.Sum256([]byte{byte(i)})
				outboundIDs[i] = outboundID[:]
			}
			hb := &Heartbeat{NetworkID: networkID, OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: make([][]byte, 0)}
			// message = tlv header + packet
			// ParseHeartbeat() expects only the packet, hence serializedHb[tlvHeaderLength:]
			serializedHb, _ := NewHeartbeatMessage(hb)
			// we set the count to HeartbeatMaxOutboundPeersCount but we only have HeartbeatMaxOutboundPeersCount - 1
			// actually serialized in the packet
			serializedHb[tlvHeaderLength+HeartbeatPacketMinSize+len(networkID)-1] = HeartbeatMaxOutboundPeersCount
			return testcase{index: 4, source: serializedHb[tlvHeaderLength:], expected: nil, err: ErrMalformedPacket}
		}(),
		// err, doesn't reach minimum packet size
		func() testcase {
			return testcase{index: 5, source: make([]byte, HeartbeatPacketMinSize-1), expected: nil, err: ErrMalformedPacket}
		}(),
		// err, exceeds maximum packet size
		func() testcase {
			return testcase{index: 6, source: make([]byte, HeartbeatPacketMaxSize+1), expected: nil, err: ErrMalformedPacket}
		}(),
		// err, wrong byte length after minimum packet size offset,
		// this emulates the minimum data to be correct but then the remaining bytes
		// length not confirming to the size of IDs
		func() testcase {
			// +1 is needed bc of the variable length networkID, that is not part of HeartbeatPacketMinSize
			serialized := make([]byte, HeartbeatPacketMinSize+1+4)
			serialized[tlvHeaderLength] = 1
			return testcase{index: 7, source: serialized[tlvHeaderLength:], expected: nil, err: ErrMalformedPacket}
		}(),
		// err networkIDLength is zero
		func() testcase {
			hb := &Heartbeat{NetworkID: networkID, OwnID: ownID[:], OutboundIDs: make([][]byte, 0), InboundIDs: make([][]byte, 0)}
			// ParseHeartbeat() expects only the packet, hence serializedHb[tlvHeaderLength:]
			serializedHb, _ := NewHeartbeatMessage(hb)
			// set networkIDByteSize to 0
			serializedHb[tlvHeaderLength] = 0
			return testcase{index: 8, source: serializedHb[tlvHeaderLength:], expected: nil, err: ErrInvalidHeartbeatNetworkVersion}
		}(),
		// err network ID does not have the correct form (missing v)
		func() testcase {
			hb := &Heartbeat{NetworkID: []byte("0.2.1"), OwnID: ownID[:], OutboundIDs: make([][]byte, 0), InboundIDs: make([][]byte, 0)}
			// message = tlv header + packet
			// ParseHeartbeat() expects only the packet, hence serializedHb[tlvHeaderLength:]
			serializedHb, _ := NewHeartbeatMessage(hb)
			return testcase{index: 9, source: serializedHb[tlvHeaderLength:], expected: nil, err: ErrInvalidHeartbeatNetworkVersion}
		}(),
		// receive an "old" heartbeat packet
		func() testcase {
			outboundIDs := make([][]byte, HeartbeatMaxOutboundPeersCount)
			inboundIDs := make([][]byte, HeartbeatMaxInboundPeersCount)
			for i := 0; i < HeartbeatMaxOutboundPeersCount; i++ {
				outboundID := sha256.Sum256([]byte{byte(i)})
				inboundID := sha256.Sum256([]byte{byte(i + HeartbeatMaxOutboundPeersCount)})
				outboundIDs[i] = outboundID[:]
				inboundIDs[i] = inboundID[:]
			}
			hb := &Heartbeat{NetworkID: networkID, OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			// message = tlv header + packet
			serializedHb, _ := NewHeartbeatMessage(hb)
			// heartbeat without network ID: cut the network id size byte, and network ID
			serializedHb = serializedHb[tlvHeaderLength+1+len(networkID):]
			return testcase{index: 10, source: serializedHb, expected: nil, err: ErrInvalidHeartbeatNetworkVersion}
		}(),
	}

	for _, testCase := range testCases {
		hb, err := ParseHeartbeat(testCase.source)
		if testCase.err != nil {
			require.True(t, errors.Is(err, testCase.err))
			continue
		}
		require.NoError(t, err, "heartbeat should have been parsed successfully")
		assert.EqualValues(t, *testCase.expected, *hb, "expected heartbeats to be equal")
	}
}
