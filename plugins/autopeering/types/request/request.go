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
	Issuer    *peer.Peer
	Signature [SIGNATURE_SIZE]byte
}

func Unmarshal(data []byte) (*Request, error) {
	if data[0] != MARSHALED_PACKET_HEADER || len(data) != MARSHALED_TOTAL_SIZE {
		return nil, ErrMalformedPeeringRequest
	}

	peeringRequest := &Request{}

	if unmarshaledPeer, err := peer.Unmarshal(data[ISSUER_START:ISSUER_END]); err != nil {
		return nil, err
	} else {
		peeringRequest.Issuer = unmarshaledPeer
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
	peeringResponse := &response.Response{
		Type:   response.TYPE_ACCEPT,
		Issuer: ownpeer.INSTANCE,
		Peers:  peers,
	}
	peeringResponse.Sign()

	if _, err := this.Issuer.Send(peeringResponse.Marshal(), types.PROTOCOL_TYPE_TCP, false); err != nil {
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
	peeringResponse.Sign()

	if _, err := this.Issuer.Send(peeringResponse.Marshal(), types.PROTOCOL_TYPE_TCP, false); err != nil {
		return err
	}

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
	result := make([]byte, MARSHALED_TOTAL_SIZE)

	result[PACKET_HEADER_START] = MARSHALED_PACKET_HEADER
	copy(result[ISSUER_START:ISSUER_END], this.Issuer.Marshal())
	copy(result[SIGNATURE_START:SIGNATURE_END], this.Signature[:SIGNATURE_SIZE])

	return result
}
