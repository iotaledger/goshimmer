package peer

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"

	"github.com/golang/protobuf/proto"
	pb "github.com/iotaledger/goshimmer/packages/autopeering/peer/proto"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
)

// PublicKey is the type of Ed25519 public keys used for peers.
type PublicKey ed25519.PublicKey

// Peer defines the immutable data of a peer.
type Peer struct {
	id        ID              // comparable node identifier
	publicKey PublicKey       // public key used to verify signatures
	services  *service.Record // unmodifiable services supported by the peer
}

// ID returns the identifier of the peer.
func (p *Peer) ID() ID {
	return p.id
}

// PublicKey returns the public key of the peer.
func (p *Peer) PublicKey() PublicKey {
	return p.publicKey
}

// Network returns the autopeering network of the peer.
func (p *Peer) Network() string {
	return p.services.Get(service.PeeringKey).Network()
}

// Address returns the autopeering address of a peer.
func (p *Peer) Address() string {
	return p.services.Get(service.PeeringKey).String()
}

// Services returns the supported services of the peer.
func (p *Peer) Services() service.Service {
	return p.services
}

// String returns a string representation of the peer.
func (p *Peer) String() string {
	u := url.URL{
		Scheme: "peer",
		User:   url.User(base64.StdEncoding.EncodeToString(p.PublicKey())),
		Host:   p.Address(),
	}
	return u.String()
}

// SignedData is an interface wrapper around data with key and signature.
type SignedData interface {
	GetData() []byte
	GetPublicKey() []byte
	GetSignature() []byte
}

// RecoverKeyFromSignedData validates and returns the key that was used to sign the data.
func RecoverKeyFromSignedData(m SignedData) (PublicKey, error) {
	return recoverKey(m.GetPublicKey(), m.GetData(), m.GetSignature())
}

// NewPeer creates a new unmodifiable peer.
func NewPeer(publicKey PublicKey, services service.Service) *Peer {
	if services.Get(service.PeeringKey) == nil {
		panic("need peering service")
	}

	return &Peer{
		id:        publicKey.ID(),
		publicKey: publicKey,
		services:  services.CreateRecord(),
	}
}

// ToProto encodes a given peer into a proto buffer Peer message
func (p *Peer) ToProto() *pb.Peer {
	return &pb.Peer{
		PublicKey: p.publicKey,
		Services:  p.services.ToProto(),
	}
}

// FromProto decodes a given proto buffer Peer message (in) and returns the corresponding Peer.
func FromProto(in *pb.Peer) (*Peer, error) {
	if l := len(in.GetPublicKey()); l != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid key length: %d, need %d", l, ed25519.PublicKeySize)
	}
	services, err := service.FromProto(in.GetServices())
	if err != nil {
		return nil, err
	}
	if services.Get(service.PeeringKey) == nil {
		return nil, errors.New("need peering service")
	}

	return NewPeer(in.GetPublicKey(), services), nil
}

// Marshal serializes a given Peer (p) into a slice of bytes.
func (p *Peer) Marshal() ([]byte, error) {
	return proto.Marshal(p.ToProto())
}

// Unmarshal de-serializes a given slice of bytes (data) into a Peer.
func Unmarshal(data []byte) (*Peer, error) {
	s := &pb.Peer{}
	if err := proto.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return FromProto(s)
}

func recoverKey(key, data, sig []byte) (PublicKey, error) {
	if l := len(key); l != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid key length: %d, need %d", l, ed25519.PublicKeySize)
	}
	if l := len(sig); l != ed25519.SignatureSize {
		return nil, fmt.Errorf("invalid signature length: %d, need %d", l, ed25519.SignatureSize)
	}
	if !ed25519.Verify(key, data, sig) {
		return nil, errors.New("invalid signature")
	}

	publicKey := make([]byte, ed25519.PublicKeySize)
	copy(publicKey, key)
	return publicKey, nil
}
