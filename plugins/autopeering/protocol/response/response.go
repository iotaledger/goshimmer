package response

import (
    "bytes"
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "github.com/pkg/errors"
)

type Response struct {
    Type      Type
    Issuer    *peer.Peer
    Peers     []*peer.Peer
    Signature [MARSHALLED_SIGNATURE_SIZE]byte
}

func Unmarshal(data []byte) (*Response, error) {
    if data[0] != MARHSALLED_PACKET_HEADER || len(data) < MARSHALLED_TOTAL_SIZE {
        return nil, errors.New("malformed peering response")
    }

    peeringResponse := &Response{
        Type:  data[MARSHALLED_TYPE_START],
        Peers: make([]*peer.Peer, 0),
    }

    if unmarshalledPeer, err := peer.Unmarshal(data[MARSHALLED_ISSUER_START:MARSHALLED_ISSUER_END]); err != nil {
        return nil, err
    } else {
        peeringResponse.Issuer = unmarshalledPeer
    }

    for i := 0; i < MARSHALLED_PEERS_AMOUNT; i++ {
        PEERING_RESPONSE_MARSHALLED_PEER_START := MARSHALLED_PEERS_START + (i * MARSHALLED_PEER_SIZE)
        PEERING_RESPONSE_MARSHALLED_PEER_END := PEERING_RESPONSE_MARSHALLED_PEER_START + MARSHALLED_PEER_SIZE

        if data[PEERING_RESPONSE_MARSHALLED_PEER_START] == 1 {
            peer, err := peer.Unmarshal(data[PEERING_RESPONSE_MARSHALLED_PEER_START+1 : PEERING_RESPONSE_MARSHALLED_PEER_END])
            if err != nil {
                return nil, err
            }

            peeringResponse.Peers = append(peeringResponse.Peers, peer)
        }
    }

    if issuer, err := identity.FromSignedData(data[:MARSHALLED_SIGNATURE_START], data[MARSHALLED_SIGNATURE_START:]); err != nil {
        return nil, err
    } else {
        if !bytes.Equal(issuer.Identifier, peeringResponse.Issuer.Identity.Identifier) {
            return nil, ErrInvalidSignature
        }
    }
    copy(peeringResponse.Signature[:], data[MARSHALLED_SIGNATURE_START:MARSHALLED_SIGNATURE_END])

    return peeringResponse, nil
}

func (this *Response) Sign() *Response {
    dataToSign := this.Marshal()[:MARSHALLED_SIGNATURE_START]
    if signature, err := this.Issuer.Identity.Sign(dataToSign); err != nil {
        panic(err)
    } else {
        copy(this.Signature[:], signature)
    }

    return this
}

func (this *Response) Marshal() []byte {
    result := make([]byte, MARSHALLED_TOTAL_SIZE)

    result[MARSHALLED_PACKET_HEADER_START] = MARHSALLED_PACKET_HEADER
    result[MARSHALLED_TYPE_START] = this.Type

    copy(result[MARSHALLED_ISSUER_START:MARSHALLED_ISSUER_END], this.Issuer.Marshal())

    for i, peer := range this.Peers {
        PEERING_RESPONSE_MARSHALLED_PEER_START := MARSHALLED_PEERS_START + (i * MARSHALLED_PEER_SIZE)
        PEERING_RESPONSE_MARSHALLED_PEER_END := PEERING_RESPONSE_MARSHALLED_PEER_START + MARSHALLED_PEER_SIZE

        result[PEERING_RESPONSE_MARSHALLED_PEER_START] = 1
        copy(result[PEERING_RESPONSE_MARSHALLED_PEER_START+1:PEERING_RESPONSE_MARSHALLED_PEER_END], peer.Marshal()[:MARSHALLED_PEER_SIZE-1])
    }

    copy(result[MARSHALLED_SIGNATURE_START:MARSHALLED_SIGNATURE_END], this.Signature[:MARSHALLED_SIGNATURE_SIZE])

    return result
}
