package peer

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/wollac/autopeering/identity"
	pb "github.com/wollac/autopeering/proto"
)

type PacketBuilder struct {
	id *identity.PrivateIdentity
}

func NewPacketBuilder(id *identity.PrivateIdentity) *PacketBuilder {
	return &PacketBuilder{
		id: id,
	}
}

func (p *PacketBuilder) Encode(message *pb.MessageWrapper) (*pb.Packet, error) {
	data, err := proto.Marshal(message)
	if err != nil {
		return nil, errors.Wrap(err, "encode")
	}
	sig := p.id.Sign(data)

	packet := &pb.Packet{
		PublicKey: p.id.PublicKey,
		Signature: sig,
		Data:      data,
	}
	return packet, nil
}

func Decode(packet *pb.Packet, message *pb.MessageWrapper) (*identity.Identity, error) {

	issuer, err := identity.NewIdentity(packet.GetPublicKey())
	if err != nil {
		return nil, errors.Wrap(err, "invalid identity")
	}

	data := packet.GetData()
	if !issuer.VerifySignature(data, packet.GetSignature()) {
		return nil, errors.Wrap(err, "invalid signature")
	}

	if err := proto.Unmarshal(data, message); err != nil {
		return nil, errors.Wrap(err, "invalid data")
	}
	return issuer, nil
}
