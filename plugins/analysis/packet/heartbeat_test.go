package packet_test

import (
	"crypto/sha256"
	"errors"
	"testing"

	. "github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ownID = sha256.Sum256([]byte{'A'})

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
				return &Heartbeat{OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
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
				return &Heartbeat{OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			}(),
			err: nil,
		},
		// ok, packet with no peer IDs (excluding own ID) is legit too
		{
			hb: func() *Heartbeat {
				outboundIDs := make([][]byte, 0)
				inboundIDs := make([][]byte, 0)
				return &Heartbeat{OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			}(),
			err: nil,
		},
		// err, has no own peer ID
		{
			hb: func() *Heartbeat {
				return &Heartbeat{OwnID: nil, OutboundIDs: make([][]byte, 0), InboundIDs: make([][]byte, 0)}
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
				return &Heartbeat{OwnID: ownID[:], OutboundIDs: outboundIDs}
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
				return &Heartbeat{OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			}(),
			err: ErrInvalidHeartbeat,
		},
	}

	for _, testCase := range testCases {
		hb := testCase.hb
		serializedHb, err := NewHeartbeatMessage(hb)
		if testCase.err != nil {
			require.True(t, errors.Is(err, testCase.err))
			continue
		}

		require.NoError(t, err, "heartbeat should have been serialized successfully")
		assert.EqualValues(t, HeartbeatPacketHeader, serializedHb[0], "expected heartbeat header value as first byte")
		assert.EqualValues(t, hb.OwnID[:], serializedHb[HeartbeatPacketHeaderSize:HeartbeatPacketHeaderSize+HeartbeatPacketPeerIDSize], "expected own peer id to be within range of %d:%d", HeartbeatPacketHeaderSize, HeartbeatPacketPeerIDSize)
		assert.EqualValues(t, len(hb.OutboundIDs), serializedHb[HeartbeatPacketHeaderSize+HeartbeatPacketPeerIDSize], "expected outbound IDs count of %d", len(hb.OutboundIDs))

		// after the outbound IDs count, the outbound IDs are serialized
		offset := HeartbeatPacketMinSize
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
	type testcase struct {
		source   []byte
		expected *Heartbeat
		err      error
	}
	testCases := []testcase{
		// ok
		func() testcase {
			hb := &Heartbeat{OwnID: ownID[:], OutboundIDs: make([][]byte, 0), InboundIDs: make([][]byte, 0)}
			serializedHb, _ := NewHeartbeatMessage(hb)
			return testcase{source: serializedHb, expected: hb, err: nil}
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
			hb := &Heartbeat{OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			serializedHb, _ := NewHeartbeatMessage(hb)
			return testcase{source: serializedHb, expected: hb, err: nil}
		}(),
		// err, modify wrong packet header
		func() testcase {
			hb := &Heartbeat{OwnID: ownID[:], OutboundIDs: make([][]byte, 0), InboundIDs: make([][]byte, 0)}
			serializedHb, _ := NewHeartbeatMessage(hb)
			serializedHb[0] = 0xff
			return testcase{source: serializedHb, expected: nil, err: ErrMalformedPacket}
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
			hb := &Heartbeat{OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
			serializedHb, _ := NewHeartbeatMessage(hb)
			// add an additional peer id
			serializedHb = append(serializedHb, ownID[:]...)
			return testcase{source: serializedHb, expected: hb, err: ErrMalformedPacket}
		}(),
		// err, exceeds max outbound peer IDs
		func() testcase {
			outboundIDs := make([][]byte, HeartbeatMaxOutboundPeersCount+1)
			for i := 0; i < HeartbeatMaxInboundPeersCount+1; i++ {
				outboundID := sha256.Sum256([]byte{byte(i)})
				outboundIDs[i] = outboundID[:]
			}
			hb := &Heartbeat{OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: make([][]byte, 0)}
			serializedHb, _ := NewHeartbeatMessage(hb)
			return testcase{source: serializedHb, expected: hb, err: ErrMalformedPacket}
		}(),
		// err, advertised outbound ID count is bigger than remaining data
		func() testcase {
			outboundIDs := make([][]byte, HeartbeatMaxOutboundPeersCount-1)
			for i := 0; i < HeartbeatMaxOutboundPeersCount-1; i++ {
				outboundID := sha256.Sum256([]byte{byte(i)})
				outboundIDs[i] = outboundID[:]
			}
			hb := &Heartbeat{OwnID: ownID[:], OutboundIDs: outboundIDs, InboundIDs: make([][]byte, 0)}
			serializedHb, _ := NewHeartbeatMessage(hb)
			// we set the count to HeartbeatMaxOutboundPeersCount but we only have HeartbeatMaxOutboundPeersCount - 1
			// actually serialized in the packet
			serializedHb[HeartbeatPacketMinSize-1] = HeartbeatMaxOutboundPeersCount
			return testcase{source: serializedHb, expected: nil, err: ErrMalformedPacket}
		}(),
		// err, doesn't reach minimum packet size
		func() testcase {
			return testcase{source: make([]byte, HeartbeatPacketMinSize-1), expected: nil, err: ErrMalformedPacket}
		}(),
		// err, exceeds maximum packet size
		func() testcase {
			return testcase{source: make([]byte, HeartbeatPacketMaxSize+1), expected: nil, err: ErrMalformedPacket}
		}(),
		// err, wrong byte length after minimum packet size offset,
		// this emulates the minimum data to be correct but then the remaining bytes
		// length not confirming to the size of IDs
		func() testcase {
			return testcase{source: make([]byte, HeartbeatPacketMinSize+4), expected: nil, err: ErrMalformedPacket}
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
