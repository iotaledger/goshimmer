package drop

import (
	"bytes"

	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
)

type Drop struct {
	Issuer    *peer.Peer
	Signature [MARSHALED_SIGNATURE_SIZE]byte
}

func Unmarshal(data []byte) (*Drop, error) {
	if data[0] != MARSHALED_PACKET_HEADER || len(data) != MARSHALED_TOTAL_SIZE {
		return nil, ErrMalformedDropMessage
	}

	ping := &Drop{}

	if unmarshaledPeer, err := peer.Unmarshal(data[MARSHALED_ISSUER_START:MARSHALED_ISSUER_END]); err != nil {
		return nil, err
	} else {
		ping.Issuer = unmarshaledPeer
	}
	if err := saltmanager.CheckSalt(ping.Issuer.GetSalt()); err != nil {
		return nil, err
	}

	if issuer, err := identity.FromSignedData(data[:MARSHALED_SIGNATURE_START], data[MARSHALED_SIGNATURE_START:]); err != nil {
		return nil, err
	} else {
		if !bytes.Equal(issuer.Identifier, ping.Issuer.GetIdentity().Identifier) {
			return nil, ErrInvalidSignature
		}
	}
	copy(ping.Signature[:], data[MARSHALED_SIGNATURE_START:MARSHALED_SIGNATURE_END])

	return ping, nil
}

func (ping *Drop) Marshal() []byte {
	result := make([]byte, MARSHALED_TOTAL_SIZE)

	result[PACKET_HEADER_START] = MARSHALED_PACKET_HEADER
	copy(result[MARSHALED_ISSUER_START:MARSHALED_ISSUER_END], ping.Issuer.Marshal())
	copy(result[MARSHALED_SIGNATURE_START:MARSHALED_SIGNATURE_END], ping.Signature[:MARSHALED_SIGNATURE_SIZE])

	return result
}

func (this *Drop) Sign() {
	if signature, err := this.Issuer.GetIdentity().Sign(this.Marshal()[:MARSHALED_SIGNATURE_START]); err != nil {
		panic(err)
	} else {
		copy(this.Signature[:], signature)
	}
}
