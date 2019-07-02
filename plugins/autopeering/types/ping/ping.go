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
	Signature [MARSHALED_SIGNATURE_SIZE]byte
}

func Unmarshal(data []byte) (*Ping, error) {
	if data[0] != MARSHALED_PACKET_HEADER || len(data) != MARSHALED_TOTAL_SIZE {
		return nil, ErrMalformedPing
	}

	ping := &Ping{
		Neighbors: make(peerlist.PeerList, 0),
	}

	if unmarshaledPeer, err := peer.Unmarshal(data[MARSHALED_ISSUER_START:MARSHALED_ISSUER_END]); err != nil {
		return nil, err
	} else {
		ping.Issuer = unmarshaledPeer
	}
	if err := saltmanager.CheckSalt(ping.Issuer.Salt); err != nil {
		return nil, err
	}

	offset := MARSHALED_PEERS_START
	for i := 0; i < constants.NEIGHBOR_COUNT; i++ {
		if data[offset] == 1 {
			if unmarshaledPing, err := peer.Unmarshal(data[offset+1 : offset+MARSHALED_PEER_ENTRY_SIZE]); err != nil {
				return nil, err
			} else {
				ping.Neighbors = append(ping.Neighbors, unmarshaledPing)
			}
		}

		offset += MARSHALED_PEER_ENTRY_SIZE
	}

	if issuer, err := identity.FromSignedData(data[:MARSHALED_SIGNATURE_START], data[MARSHALED_SIGNATURE_START:]); err != nil {
		return nil, err
	} else {
		if !bytes.Equal(issuer.Identifier, ping.Issuer.Identity.Identifier) {
			return nil, ErrInvalidSignature
		}
	}
	copy(ping.Signature[:], data[MARSHALED_SIGNATURE_START:MARSHALED_SIGNATURE_END])

	return ping, nil
}

func (ping *Ping) Marshal() []byte {
	result := make([]byte, MARSHALED_TOTAL_SIZE)

	result[PACKET_HEADER_START] = MARSHALED_PACKET_HEADER
	copy(result[MARSHALED_ISSUER_START:MARSHALED_ISSUER_END], ping.Issuer.Marshal())
	for i, neighbor := range ping.Neighbors {
		entryStartOffset := MARSHALED_PEERS_START + i*MARSHALED_PEER_ENTRY_SIZE

		result[entryStartOffset] = 1

		copy(result[entryStartOffset+1:entryStartOffset+MARSHALED_PEER_ENTRY_SIZE], neighbor.Marshal())
	}
	copy(result[MARSHALED_SIGNATURE_START:MARSHALED_SIGNATURE_END], ping.Signature[:MARSHALED_SIGNATURE_SIZE])

	return result
}

func (this *Ping) Sign() {
	if signature, err := this.Issuer.Identity.Sign(this.Marshal()[:MARSHALED_SIGNATURE_START]); err != nil {
		panic(err)
	} else {
		copy(this.Signature[:], signature)
	}
}
