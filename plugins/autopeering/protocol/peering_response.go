package protocol

import (
    "bytes"
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/pkg/errors"
)

type PeeringResponse struct {
    Type      ResponseType
    Issuer    *Peer
    Peers     []*Peer
    Signature [PEERING_RESPONSE_MARSHALLED_SIGNATURE_SIZE]byte
}

func UnmarshalPeeringResponse(data []byte) (*PeeringResponse, error) {
    if len(data) < PEERING_RESPONSE_MARSHALLED_TOTAL_SIZE {
        return nil, errors.New("size of marshalled peering response is too small")
    }

    peeringResponse := &PeeringResponse{
        Type:  data[PEERING_RESPONSE_MARSHALLED_TYPE_START],
        Peers: make([]*Peer, 0),
    }

    if unmarshalledPeer, err := UnmarshalPeer(data[PEERING_RESPONSE_MARSHALLED_ISSUER_START:PEERING_RESPONSE_MARSHALLED_ISSUER_END]); err != nil {
        return nil, err
    } else {
        peeringResponse.Issuer = unmarshalledPeer
    }

    for i := 0; i < PEERING_RESPONSE_MARSHALLED_PEERS_AMOUNT; i++ {
        PEERING_RESPONSE_MARSHALLED_PEER_START := PEERING_RESPONSE_MARSHALLED_PEERS_START + (i * PEERING_RESPONSE_MARSHALLED_PEER_SIZE)
        PEERING_RESPONSE_MARSHALLED_PEER_END := PEERING_RESPONSE_MARSHALLED_PEER_START + PEERING_RESPONSE_MARSHALLED_PEER_SIZE

        if data[PEERING_RESPONSE_MARSHALLED_PEER_START] == 1 {
            peer, err := UnmarshalPeer(data[PEERING_RESPONSE_MARSHALLED_PEER_START+1 : PEERING_RESPONSE_MARSHALLED_PEER_END])
            if err != nil {
                return nil, err
            }

            peeringResponse.Peers = append(peeringResponse.Peers, peer)
        }
    }

    if issuer, err := identity.FromSignedData(data[:PEERING_RESPONSE_MARSHALLED_SIGNATURE_START], data[PEERING_RESPONSE_MARSHALLED_SIGNATURE_START:]); err != nil {
        return nil, err
    } else {
        if !bytes.Equal(issuer.Identifier, peeringResponse.Issuer.Identity.Identifier) {
            return nil, ErrInvalidSignature
        }
    }
    copy(peeringResponse.Signature[:], data[PEERING_RESPONSE_MARSHALLED_SIGNATURE_START:PEERING_RESPONSE_MARSHALLED_SIGNATURE_END])

    return peeringResponse, nil
}

func (this *PeeringResponse) Sign() *PeeringResponse {
    dataToSign := this.Marshal()[:PEERING_RESPONSE_MARSHALLED_SIGNATURE_START]
    if signature, err := this.Issuer.Identity.Sign(dataToSign); err != nil {
        panic(err)
    } else {
        copy(this.Signature[:], signature)
    }

    return this
}

func (this *PeeringResponse) Marshal() []byte {
    result := make([]byte, PEERING_RESPONSE_MARSHALLED_TOTAL_SIZE)

    result[PEERING_RESPONSE_MARSHALLED_TYPE_START] = this.Type

    copy(result[PEERING_RESPONSE_MARSHALLED_ISSUER_START:PEERING_RESPONSE_MARSHALLED_ISSUER_END], this.Issuer.Marshal())

    for i, peer := range this.Peers {
        PEERING_RESPONSE_MARSHALLED_PEER_START := PEERING_RESPONSE_MARSHALLED_PEERS_START + (i * PEERING_RESPONSE_MARSHALLED_PEER_SIZE)
        PEERING_RESPONSE_MARSHALLED_PEER_END := PEERING_RESPONSE_MARSHALLED_PEER_START + PEERING_RESPONSE_MARSHALLED_PEER_SIZE

        result[PEERING_RESPONSE_MARSHALLED_PEER_START] = 1
        copy(result[PEERING_RESPONSE_MARSHALLED_PEER_START+1:PEERING_RESPONSE_MARSHALLED_PEER_END], peer.Marshal()[:PEERING_RESPONSE_MARSHALLED_PEER_SIZE-1])
    }

    copy(result[PEERING_RESPONSE_MARSHALLED_SIGNATURE_START:PEERING_RESPONSE_MARSHALLED_SIGNATURE_END], this.Signature[:PEERING_RESPONSE_MARSHALLED_SIGNATURE_SIZE])

    return result
}
