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
	if data[0] != MARSHALLED_PACKET_HEADER || len(data) != MARSHALLED_TOTAL_SIZE {
		return nil, ErrMalformedDropMessage
	}

	// check the signature
	signer, err := identity.FromSignedData(data)
	if err != nil {
		return nil, ErrInvalidSignature
	}

	ping := &Drop{}

	if unmarshalledPeer, err := peer.Unmarshal(data[MARSHALLED_ISSUER_START:MARSHALLED_ISSUER_END]); err != nil {
		return nil, err
	} else {
		ping.Issuer = unmarshalledPeer
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
	msg := make([]byte, MARSHALLED_SIGNATURE_SIZE)

	msg[PACKET_HEADER_START] = MARSHALLED_PACKET_HEADER
	copy(msg[MARSHALLED_ISSUER_START:MARSHALLED_ISSUER_END], ping.Issuer.Marshal())

	return ping.Issuer.Identity.AddSignature(msg)
}
