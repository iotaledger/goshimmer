package protocol

import (
    "bytes"
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/plugins/autopeering/salt"
    "github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
    "time"
)

type PeeringRequest struct {
    Issuer    *Peer
    Salt      *salt.Salt
    Signature [SIGNATURE_SIZE]byte
}

func UnmarshalPeeringRequest(data []byte) (*PeeringRequest, error) {
    if data[0] != PACKET_HEADER || len(data) != PEERING_REQUEST_MARSHALLED_TOTAL_SIZE {
        return nil, ErrMalformedPeeringRequest
    }

    peeringRequest := &PeeringRequest{}

    if unmarshalledPeer, err := UnmarshalPeer(data[ISSUER_START:ISSUER_END]); err != nil {
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
    if peeringRequest.Salt.ExpirationTime.Before(now) {
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

func (this *PeeringRequest) Sign() {
    dataToSign := this.Marshal()[:SIGNATURE_START]
    if signature, err := this.Issuer.Identity.Sign(dataToSign); err != nil {
        panic(err)
    } else {
        copy(this.Signature[:], signature)
    }
}

func (this *PeeringRequest) Marshal() []byte {
    result := make([]byte, PEERING_REQUEST_MARSHALLED_TOTAL_SIZE)

    result[PACKET_HEADER_START] = PACKET_HEADER
    copy(result[ISSUER_START:ISSUER_END], this.Issuer.Marshal())
    copy(result[SALT_START:SALT_END], this.Salt.Marshal())
    copy(result[SIGNATURE_START:SIGNATURE_END], this.Signature[:SIGNATURE_SIZE])

    return result
}
