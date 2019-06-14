package ping

import (
	"bytes"

	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
	"github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
)

type Ping struct {
	Issuer    *peer.Peer
	Neighbors peerlist.PeerList
}

func Unmarshal(data []byte) (*Ping, error) {
	if data[0] != MARSHALLED_PACKET_HEADER || len(data) != MARSHALLED_TOTAL_SIZE {
		return nil, ErrMalformedPing
	}

	// check the signature
	signer, err := identity.FromSignedData(data)
	if err != nil {
		return nil, ErrInvalidSignature
	}

	// unmarshal the actual ping data
	ping := &Ping{
		Neighbors: make(peerlist.PeerList, 0),
	}

	if unmarshalledPeer, err := peer.Unmarshal(data[MARSHALLED_ISSUER_START:MARSHALLED_ISSUER_END]); err != nil {
		return nil, ErrMalformedPing
	} else {
		ping.Issuer = unmarshalledPeer
	}

	// the ping issuer must match the signer
	if !bytes.Equal(signer.Identifier, ping.Issuer.Identity.Identifier) {
		return nil, ErrInvalidSignature
	}

	// store the signer as it also contains the public key
	ping.Issuer.Identity = signer

	if err := saltmanager.CheckSalt(ping.Issuer.Salt); err != nil {
		return nil, err
	}

	offset := MARSHALLED_PEERS_START
	for i := 0; i < constants.NEIGHBOR_COUNT; i++ {
		if data[offset] == 1 {
			if unmarshalledPing, err := peer.Unmarshal(data[offset+1 : offset+MARSHALLED_PEER_ENTRY_SIZE]); err != nil {
				return nil, err
			} else {
				ping.Neighbors = append(ping.Neighbors, unmarshalledPing)
			}
		}

		offset += MARSHALLED_PEER_ENTRY_SIZE
	}

	return ping, nil
}

func (ping *Ping) Marshal() []byte {
	msg := make([]byte, MARSHALLED_SIGNATURE_START)

	msg[PACKET_HEADER_START] = MARSHALLED_PACKET_HEADER
	copy(msg[MARSHALLED_ISSUER_START:MARSHALLED_ISSUER_END], ping.Issuer.Marshal())

	for i, neighbor := range ping.Neighbors {
		entryStartOffset := MARSHALLED_PEERS_START + i*MARSHALLED_PEER_ENTRY_SIZE

		msg[entryStartOffset] = 1

		copy(msg[entryStartOffset+1:entryStartOffset+MARSHALLED_PEER_ENTRY_SIZE], neighbor.Marshal())
	}

	// return the signed message
	return ping.Issuer.Identity.AddSignature(msg)
}
