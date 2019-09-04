package peer

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/golang/protobuf/proto"
	pb "github.com/wollac/autopeering/peer/proto"
	"golang.org/x/crypto/ed25519"
)

// PublicKey is the type of Ed25519 public keys used for peers.
type PublicKey ed25519.PublicKey

// Peer defines the immutable data of a peer.
type Peer struct {
	id        ID        // comparable node identifier
	publicKey PublicKey // public key used to verify signatures
	address   string    // address of a peer ("127.0.0.1:8000")
}

// ID returns the node identifier.
func (p *Peer) ID() ID {
	return p.id
}

// Address returns the address of a peer.
func (p *Peer) Address() string {
	return p.address
}

// String returns a string representation of the peer.
func (p *Peer) String() string {
	u := url.URL{
		Scheme: "peer",
		User:   url.User(fmt.Sprintf("%x", p.publicKey)),
		Host:   p.address,
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

// NewPeer creates a new minimal peer.
func NewPeer(publicKey PublicKey, address string) *Peer {
	return &Peer{
		id:        publicKey.ID(),
		publicKey: publicKey,
		address:   address,
	}
}

// ToProto encodes a given peer into a proto buffer Peer message
func (p *Peer) ToProto() *pb.Peer {
	return &pb.Peer{
		PublicKey: p.publicKey,
		Address:   p.address,
	}
}

// FromProto decodes a given proto buffer Peer message (in) and returns the corresponding Peer.
func FromProto(in *pb.Peer) (*Peer, error) {
	if l := len(in.GetPublicKey()); l != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid key length: %d, need %d", l, ed25519.PublicKeySize)
	}
	return NewPeer(in.GetPublicKey(), in.GetAddress()), nil
}

// Marshal serializes a given Peer (p) into a slice of bytes.
func (p *Peer) Marshal() ([]byte, error) {
	return proto.Marshal(p.ToProto())
}

// Unmarshal deserializes a given slice of bytes (data) into a Peer.
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
	return PublicKey(publicKey), nil
}
