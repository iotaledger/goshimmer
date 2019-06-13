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
	Signature [MARSHALLED_SIGNATURE_SIZE]byte
}

func Unmarshal(data []byte) (*Ping, error) {
	if data[0] != MARSHALLED_PACKET_HEADER || len(data) != MARSHALLED_TOTAL_SIZE {
		return nil, ErrMalformedPing
	}

	ping := &Ping{
		Neighbors: make(peerlist.PeerList, 0),
	}

	if unmarshalledPeer, err := peer.Unmarshal(data[MARSHALLED_ISSUER_START:MARSHALLED_ISSUER_END]); err != nil {
		return nil, err
	} else {
		ping.Issuer = unmarshalledPeer
	}
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

	if issuer, err := identity.FromSignedData(data[:MARSHALLED_SIGNATURE_START], data[MARSHALLED_SIGNATURE_START:]); err != nil {
		return nil, err
	} else {
		if !bytes.Equal(issuer.Identifier, ping.Issuer.Identity.Identifier) {
			return nil, ErrInvalidSignature
		}
	}
	copy(ping.Signature[:], data[MARSHALLED_SIGNATURE_START:MARSHALLED_SIGNATURE_END])

	return ping, nil
}

func (ping *Ping) Marshal() []byte {
	result := make([]byte, MARSHALLED_TOTAL_SIZE)

	result[PACKET_HEADER_START] = MARSHALLED_PACKET_HEADER
	copy(result[MARSHALLED_ISSUER_START:MARSHALLED_ISSUER_END], ping.Issuer.Marshal())
	for i, neighbor := range ping.Neighbors {
		entryStartOffset := MARSHALLED_PEERS_START + i*MARSHALLED_PEER_ENTRY_SIZE

		result[entryStartOffset] = 1

		copy(result[entryStartOffset+1:entryStartOffset+MARSHALLED_PEER_ENTRY_SIZE], neighbor.Marshal())
	}
	copy(result[MARSHALLED_SIGNATURE_START:MARSHALLED_SIGNATURE_END], ping.Signature[:MARSHALLED_SIGNATURE_SIZE])

	return result
}

func (this *Ping) Sign() {
	if signature, err := this.Issuer.Identity.Sign(this.Marshal()[:MARSHALLED_SIGNATURE_START]); err != nil {
		panic(err)
	} else {
		copy(this.Signature[:], signature)
	}
}
