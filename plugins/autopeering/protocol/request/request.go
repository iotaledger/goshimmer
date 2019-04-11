package request

import (
    "bytes"
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/response"
    "github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
    "time"
)

type Request struct {
    Issuer    *peer.Peer
    Signature [SIGNATURE_SIZE]byte
}

func Unmarshal(data []byte) (*Request, error) {
    if data[0] != MARSHALLED_PACKET_HEADER || len(data) != MARSHALLED_TOTAL_SIZE {
        return nil, ErrMalformedPeeringRequest
    }

    peeringRequest := &Request{}

    if unmarshalledPeer, err := peer.Unmarshal(data[ISSUER_START:ISSUER_END]); err != nil {
        return nil, err
    } else {
        peeringRequest.Issuer = unmarshalledPeer
    }

    now := time.Now()
    if peeringRequest.Issuer.Salt.ExpirationTime.Before(now.Add(-1 * time.Minute)) {
        return nil, ErrPublicSaltExpired
    }
    if peeringRequest.Issuer.Salt.ExpirationTime.After(now.Add(saltmanager.PUBLIC_SALT_LIFETIME + 1*time.Minute)) {
        return nil, ErrPublicSaltInvalidLifetime
    }

    if issuer, err := identity.FromSignedData(data[:SIGNATURE_START], data[SIGNATURE_START:]); err != nil {
        return nil, err
    } else {
        if !bytes.Equal(issuer.Identifier, peeringRequest.Issuer.Identity.Identifier) {
            return nil, ErrInvalidSignature
        }
    }
    copy(peeringRequest.Signature[:], data[SIGNATURE_START:SIGNATURE_END])

    return peeringRequest, nil
}

func (this *Request) Accept(peers []*peer.Peer) error {
    if _, err := this.Issuer.Connect(); err != nil {
        return err
    }

    peeringResponse := &response.Response{
        Type:   response.TYPE_ACCEPT,
        Issuer: OUTGOING_REQUEST.Issuer,
        Peers:  peers,
    }
    peeringResponse.Sign()

    if err := this.Issuer.Send(peeringResponse.Marshal(), false); err != nil {
        return err
    }

    return nil
}

func (this *Request) Reject(peers []*peer.Peer) error {
    return nil
}

func (this *Request) Sign() {
    if signature, err := this.Issuer.Identity.Sign(this.Marshal()[:SIGNATURE_START]); err != nil {
        panic(err)
    } else {
        copy(this.Signature[:], signature)
    }
}

func (this *Request) Marshal() []byte {
    result := make([]byte, MARSHALLED_TOTAL_SIZE)

    result[PACKET_HEADER_START] = MARSHALLED_PACKET_HEADER
    copy(result[ISSUER_START:ISSUER_END], this.Issuer.Marshal())
    copy(result[SIGNATURE_START:SIGNATURE_END], this.Signature[:SIGNATURE_SIZE])

    return result
}
