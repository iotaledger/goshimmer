package drop

import (
	"bytes"

	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
)

type Drop struct {
	Issuer    *peer.Peer
	Signature [MARSHALLED_SIGNATURE_SIZE]byte
}

func Unmarshal(data []byte) (*Drop, error) {
	if data[0] != MARSHALLED_PACKET_HEADER || len(data) != MARSHALLED_TOTAL_SIZE {
		return nil, ErrMalformedDropMessage
	}

	ping := &Drop{}

	if unmarshalledPeer, err := peer.Unmarshal(data[MARSHALLED_ISSUER_START:MARSHALLED_ISSUER_END]); err != nil {
		return nil, err
	} else {
		ping.Issuer = unmarshalledPeer
	}
	if err := saltmanager.CheckSalt(ping.Issuer.Salt); err != nil {
		return nil, err
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

func (ping *Drop) Marshal() []byte {
	result := make([]byte, MARSHALLED_TOTAL_SIZE)

	result[PACKET_HEADER_START] = MARSHALLED_PACKET_HEADER
	copy(result[MARSHALLED_ISSUER_START:MARSHALLED_ISSUER_END], ping.Issuer.Marshal())
	copy(result[MARSHALLED_SIGNATURE_START:MARSHALLED_SIGNATURE_END], ping.Signature[:MARSHALLED_SIGNATURE_SIZE])

	return result
}

func (this *Drop) Sign() {
	if signature, err := this.Issuer.Identity.Sign(this.Marshal()[:MARSHALLED_SIGNATURE_START]); err != nil {
		panic(err)
	} else {
		copy(this.Signature[:], signature)
	}
}
