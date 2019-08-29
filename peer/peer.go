package peer

import (
	"github.com/golang/protobuf/proto"
	"github.com/wollac/autopeering/id"
	pb "github.com/wollac/autopeering/peer/proto"
)

// Peer defines the immutable data of a peer
type Peer struct {
	Identity *id.Identity // identity of the peer (ID, StringID, PublicKey)
	Address  string       // address of a peer ("127.0.0.1:8000")
}

// ToProto encodes a given peer into a proto buffer Peer message
func ToProto(p *Peer) *pb.Peer {
	return &pb.Peer{
		PublicKey: p.Identity.PublicKey,
		Address:   p.Address,
	}
}

// FromProto decodes a given proto buffer Peer message (in) into a Peer (out)
// out MUST NOT be nil
func FromProto(in *pb.Peer, out *Peer) (err error) {
	if out == nil {
		return ErrNilInput
	}
	out.Identity, err = id.NewIdentity(in.GetPublicKey())
	if err != nil {
		return err
	}
	out.Address = in.Address
	return
}

// Marshal serializes a given Peer (p) into a slice of bytes (data)
func Marshal(p *Peer) (data []byte, err error) {
	pb := ToProto(p)
	return proto.Marshal(pb)
}

// Unmarshal deserializes a given slice of bytes (data) into a Peer (out)
// out MUST NOT be nil
func Unmarshal(data []byte, out *Peer) (err error) {
	if out == nil {
		return ErrNilInput
	}
	s := &pb.Peer{}
	err = proto.Unmarshal(data, s)
	if err != nil {
		return err
	}
	return FromProto(s, out)
}
