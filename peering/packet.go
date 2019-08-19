package peering

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/wollac/autopeering/identity"
	pb "github.com/wollac/autopeering/proto"
)

type IngressPacket struct {
	RemoteID *identity.Identity

	Message *pb.MessageWrapper
	RawData []byte
}

func Encode(id *identity.PrivateIdentity, message *pb.MessageWrapper) (*pb.Packet, error) {
	data, err := proto.Marshal(message)
	if err != nil {
		return nil, errors.Wrap(err, "encode")
	}
	sig := id.Sign(data)

	packet := &pb.Packet{
		PublicKey: id.PublicKey,
		Signature: sig,
		Data:      data,
	}
	return packet, nil
}

func Decode(packet *pb.Packet, ingress *IngressPacket) error {

	issuer, err := identity.NewIdentity(packet.GetPublicKey())
	if err != nil {
		return errors.Wrap(err, "invalid identity")
	}

	data := packet.GetData()
	if !issuer.VerifySignature(data, packet.GetSignature()) {
		return errors.Wrap(err, "invalid signature")
	}

	ingress.RemoteID = issuer
	ingress.RawData = data
	ingress.Message = &pb.MessageWrapper{}
	if err := proto.Unmarshal(data, ingress.Message); err != nil {
		return errors.Wrap(err, "invalid data")
	}

	return nil
}
