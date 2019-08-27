package peer

import (
	"github.com/golang/protobuf/proto"
	"github.com/wollac/autopeering/id"
	pb "github.com/wollac/autopeering/peer/proto"
	"github.com/wollac/autopeering/salt"
)

// Peer defines the structure of a peer
type Peer struct {
	Identity *id.Identity // identity of the peer (ID, StringID, PublicKey)
	Services ServiceMap   // map of services the peer exposes (<"autopeering":{TCP,8000}>, <"gossip":{UDP,9000}>)
	Salt     *salt.Salt   // current salt of the peer (salt, expiration time)
}

type Private struct {
	Key  []byte
	Salt *salt.Salt
}

type Own struct {
	Public  Peer
	Private Private
}

// ToProto encodes a given peer into a proto buffer Peer message
func ToProto(p *Peer) (result *pb.Peer, err error) {
	result = &pb.Peer{}
	result.PublicKey = p.Identity.PublicKey
	result.Services, err = encodeService(p.Services)
	if err != nil {
		return nil, err
	}
	result.Salt, err = salt.ToProto(p.Salt)

	return
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
	out.Services = NewServiceMap()
	err = decodeService(in.GetServices(), out.Services)
	if err != nil {
		return err
	}

	out.Salt = &salt.Salt{}
	err = salt.FromProto(in.Salt, out.Salt)

	return
}

// Marshal serializes a given Peer (p) into a slice of bytes (data)
func Marshal(p *Peer) (data []byte, err error) {
	pb, err := ToProto(p)
	if err != nil {
		return nil, err
	}
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
