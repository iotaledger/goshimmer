package response

import (
	"bytes"

	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/pkg/errors"
)

type Response struct {
	Type      Type
	Issuer    *peer.Peer
	Peers     []*peer.Peer
	Signature [MARSHALED_SIGNATURE_SIZE]byte
}

func Unmarshal(data []byte) (*Response, error) {
	if data[0] != MARHSALLED_PACKET_HEADER || len(data) < MARSHALED_TOTAL_SIZE {
		return nil, errors.New("malformed peering response")
	}

	peeringResponse := &Response{
		Type:  data[MARSHALED_TYPE_START],
		Peers: make([]*peer.Peer, 0),
	}

	if unmarshaledPeer, err := peer.Unmarshal(data[MARSHALED_ISSUER_START:MARSHALED_ISSUER_END]); err != nil {
		return nil, err
	} else {
		peeringResponse.Issuer = unmarshaledPeer
	}

	for i := 0; i < MARSHALED_PEERS_AMOUNT; i++ {
		PEERING_RESPONSE_MARSHALED_PEER_START := MARSHALED_PEERS_START + (i * MARSHALED_PEER_SIZE)
		PEERING_RESPONSE_MARSHALED_PEER_END := PEERING_RESPONSE_MARSHALED_PEER_START + MARSHALED_PEER_SIZE

		if data[PEERING_RESPONSE_MARSHALED_PEER_START] == 1 {
			peer, err := peer.Unmarshal(data[PEERING_RESPONSE_MARSHALED_PEER_START+1 : PEERING_RESPONSE_MARSHALED_PEER_END])
			if err != nil {
				return nil, err
			}

			peeringResponse.Peers = append(peeringResponse.Peers, peer)
		}
	}

	if issuer, err := identity.FromSignedData(data[:MARSHALED_SIGNATURE_START], data[MARSHALED_SIGNATURE_START:]); err != nil {
		return nil, err
	} else {
		if !bytes.Equal(issuer.Identifier, peeringResponse.Issuer.Identity.Identifier) {
			return nil, ErrInvalidSignature
		}
	}
	copy(peeringResponse.Signature[:], data[MARSHALED_SIGNATURE_START:MARSHALED_SIGNATURE_END])

	return peeringResponse, nil
}

func (this *Response) Sign() *Response {
	dataToSign := this.Marshal()[:MARSHALED_SIGNATURE_START]
	if signature, err := this.Issuer.Identity.Sign(dataToSign); err != nil {
		panic(err)
	} else {
		copy(this.Signature[:], signature)
	}

	return this
}

func (this *Response) Marshal() []byte {
	result := make([]byte, MARSHALED_TOTAL_SIZE)

	result[MARSHALED_PACKET_HEADER_START] = MARHSALLED_PACKET_HEADER
	result[MARSHALED_TYPE_START] = this.Type

	copy(result[MARSHALED_ISSUER_START:MARSHALED_ISSUER_END], this.Issuer.Marshal())

	for i, peer := range this.Peers {
		if i < constants.NEIGHBOR_COUNT {
			PEERING_RESPONSE_MARSHALED_PEER_START := MARSHALED_PEERS_START + (i * MARSHALED_PEER_SIZE)
			PEERING_RESPONSE_MARSHALED_PEER_END := PEERING_RESPONSE_MARSHALED_PEER_START + MARSHALED_PEER_SIZE

			result[PEERING_RESPONSE_MARSHALED_PEER_START] = 1
			copy(result[PEERING_RESPONSE_MARSHALED_PEER_START+1:PEERING_RESPONSE_MARSHALED_PEER_END], peer.Marshal()[:MARSHALED_PEER_SIZE-1])
		}
	}

	copy(result[MARSHALED_SIGNATURE_START:MARSHALED_SIGNATURE_END], this.Signature[:MARSHALED_SIGNATURE_SIZE])

	return result
}
