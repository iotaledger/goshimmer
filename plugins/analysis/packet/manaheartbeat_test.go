package packet

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/protocol/tlv"
	"github.com/stretchr/testify/require"
)

func dummyManaHeartbeat() *ManaHeartbeat {
	nodeMap := make(mana.NodeMap)
	networkMap := make(map[mana.Type]mana.NodeMap)
	emptyID := identity.ID{}

	nodeMap[emptyID] = 1.0
	networkMap[mana.AccessMana] = nodeMap
	networkMap[mana.ConsensusMana] = nodeMap

	online := make(map[mana.Type][]mana.Node)
	online[mana.AccessMana] = []mana.Node{{}}
	online[mana.ConsensusMana] = []mana.Node{{}}

	var pledgeEvents []mana.PledgedEvent
	var revokeEvents []mana.RevokedEvent
	pledgeEvents = append(pledgeEvents, mana.PledgedEvent{})
	revokeEvents = append(revokeEvents, mana.RevokedEvent{})

	return &ManaHeartbeat{
		Version:          banner.AppVersion,
		NetworkMap:       networkMap,
		OnlineNetworkMap: online,
		PledgeEvents:     pledgeEvents,
		RevokeEvents:     revokeEvents,
		NodeID:           emptyID,
	}
}

func TestNewManaHeart(t *testing.T) {
	hb := dummyManaHeartbeat()
	packet, err := hb.Bytes()
	require.NoError(t, err)

	hbParsed, err := ParseManaHeartbeat(packet)
	require.NoError(t, err)
	require.Equal(t, hb, hbParsed)

	tlvHeaderLength := int(tlv.HeaderMessageDefinition.MaxBytesLength)
	msg, err := NewManaHeartbeatMessage(hb)
	require.NoError(t, err)

	require.Equal(t, MessageTypeManaHeartbeat, message.Type(msg[0]))

	hbParsed, err = ParseManaHeartbeat(msg[tlvHeaderLength:])
	require.NoError(t, err)
	require.Equal(t, hb, hbParsed)
}
