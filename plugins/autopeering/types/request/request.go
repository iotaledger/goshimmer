package request

import (
	"bytes"
	"time"

	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/ownpeer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/types"
	"github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/response"
)

type Request struct {
	Issuer *peer.Peer
}

func Unmarshal(data []byte) (*Request, error) {
	if data[0] != MARSHALED_PACKET_HEADER || len(data) != MARSHALED_TOTAL_SIZE {
		return nil, ErrMalformedPeeringRequest
	}

	// check the signature
	signer, err := identity.FromSignedData(data)
	if err != nil {
		return nil, ErrInvalidSignature
	}

	peeringRequest := &Request{}

	// start unmarshaling the actual request
	if unmarshaledPeer, err := peer.Unmarshal(data[MARSHALED_ISSUER_START:MARSHALED_ISSUER_END]); err != nil {
		return nil, ErrMalformedPeeringRequest
	} else {
		peeringRequest.Issuer = unmarshaledPeer
	}

	// the request issuer must match the signer
	if !bytes.Equal(signer.Identifier, peeringRequest.Issuer.Identity.Identifier) {
		return nil, ErrInvalidSignature
	}

	// store the signer as it also contains the public key
	peeringRequest.Issuer.Identity = signer

	now := time.Now()
	if peeringRequest.Issuer.Salt.ExpirationTime.Before(now.Add(-1 * time.Minute)) {
		return nil, ErrPublicSaltExpired
	}
	if peeringRequest.Issuer.Salt.ExpirationTime.After(now.Add(saltmanager.PUBLIC_SALT_LIFETIME + 1*time.Minute)) {
		return nil, ErrPublicSaltInvalidLifetime
	}

	return peeringRequest, nil
}

func (this *Request) Accept(peers []*peer.Peer) error {
	peeringResponse := &response.Response{
		Type:   response.TYPE_ACCEPT,
		Issuer: ownpeer.INSTANCE,
		Peers:  peers,
	}

	data := peeringResponse.Marshal()
	if _, err := this.Issuer.Send(data, types.PROTOCOL_TYPE_TCP, false); err != nil {
		return err
	}

	return nil
}

func (this *Request) Reject(peers []*peer.Peer) error {
	peeringResponse := &response.Response{
		Type:   response.TYPE_REJECT,
		Issuer: ownpeer.INSTANCE,
		Peers:  peers,
	}

	data := peeringResponse.Marshal()
	if _, err := this.Issuer.Send(data, types.PROTOCOL_TYPE_TCP, false); err != nil {
		return err
	}

	return nil
}

func (this *Request) Marshal() []byte {
	msg := make([]byte, MARSHALED_SIGNATURE_START)

	msg[PACKET_HEADER_START] = MARSHALED_PACKET_HEADER
	copy(msg[MARSHALED_ISSUER_START:MARSHALED_ISSUER_END], this.Issuer.Marshal())

	// return the signed message
	return this.Issuer.Identity.AddSignature(msg)
}
