package drop

import (
	"bytes"

	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
)

type Drop struct {
	Issuer *peer.Peer
}

func Unmarshal(data []byte) (*Drop, error) {
	if data[0] != MARSHALED_PACKET_HEADER || len(data) != MARSHALED_TOTAL_SIZE {
		return nil, ErrMalformedDropMessage
	}

	// check the signature
	signer, err := identity.FromSignedData(data)
	if err != nil {
		return nil, ErrInvalidSignature
	}

	ping := &Drop{}

	if unmarshaledPeer, err := peer.Unmarshal(data[MARSHALED_ISSUER_START:MARSHALED_ISSUER_END]); err != nil {
		return nil, err
	} else {
		ping.Issuer = unmarshaledPeer
	}

	// the ping issuer must match the signer
	if !bytes.Equal(signer.Identifier, ping.Issuer.Identity.Identifier) {
		return nil, ErrInvalidSignature
	}

	// store the signer as it also contains the public key
	ping.Issuer.Identity = signer

	if err := saltmanager.CheckSalt(ping.Issuer.Salt); err != nil {
		return nil, err
	}

	return ping, nil
}

func (ping *Drop) Marshal() []byte {
	msg := make([]byte, MARSHALED_SIGNATURE_SIZE)

	msg[PACKET_HEADER_START] = MARSHALED_PACKET_HEADER
	copy(msg[MARSHALED_ISSUER_START:MARSHALED_ISSUER_END], ping.Issuer.Marshal())

	return ping.Issuer.Identity.AddSignature(msg)
}
