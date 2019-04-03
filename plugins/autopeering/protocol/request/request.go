package request

import (
    "bytes"
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/salt"
    "github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
    "time"
)

type Request struct {
    Issuer    *peer.Peer
    Salt      *salt.Salt
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

    if unmarshalledSalt, err := salt.Unmarshal(data[SALT_START:SALT_END]); err != nil {
        return nil, err
    } else {
        peeringRequest.Salt = unmarshalledSalt
    }

    now := time.Now()
    if peeringRequest.Salt.ExpirationTime.Before(now.Add(-1 * time.Minute)) {
        return nil, ErrPublicSaltExpired
    }
    if peeringRequest.Salt.ExpirationTime.After(now.Add(saltmanager.PUBLIC_SALT_LIFETIME + 1*time.Minute)) {
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
    copy(result[SALT_START:SALT_END], this.Salt.Marshal())
    copy(result[SIGNATURE_START:SIGNATURE_END], this.Signature[:SIGNATURE_SIZE])

    return result
}
