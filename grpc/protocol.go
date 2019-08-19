package grpc

import (
	context "context"
	"crypto/sha256"
	"log"
	"net"

	"github.com/pkg/errors"
	"github.com/wollac/autopeering/peering"
	pb "github.com/wollac/autopeering/proto"
	"google.golang.org/grpc"
)

type Protocol struct {
	server *grpc.Server

	onPing func(*peering.IngressPacket)
	onPong func(*peering.IngressPacket)
}

type server struct {
	p *Protocol
}

func (s *server) Ping(ctx context.Context, req *pb.Packet) (*pb.Packet, error) {
	pkt := &peering.IngressPacket{}
	if err := peering.Decode(req, pkt); err != nil {
		return nil, errors.Wrap(err, "invalid packaget")
	}
	ping := pkt.Message.GetPing()
	if ping == nil {
		return nil, errors.New("invalid packet data")
	}

	// call the callback
	if s.p.onPing != nil {
		s.p.onPing(pkt)
	}

	// create the corresponding pong
	hash := sha256.Sum256(pkt.RawData)
	pong := &pb.Pong{
		To:       ping.GetFrom(),
		PingHash: hash[:],
	}

	message := &pb.MessageWrapper{Message: &pb.MessageWrapper_Pong{Pong: pong}}
	return peering.Encode(nil, message)
}

func (p *Protocol) Start(address string) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	peeringServer := &server{p: p}
	RegisterPeeringServer(grpcServer, peeringServer)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	p.server = grpcServer
}

func (p *Protocol) Stop() {
	p.server.Stop()
	p.server = nil
}

func (p *Protocol) sendPing(conn *grpc.ClientConn, ping *pb.Packet) error {
	client := NewPeeringClient(conn)
	res, err := client.Ping(context.Background(), ping)
	if err != nil {
		return errors.Wrap(err, "error encoding packet")
	}

	pkt := &peering.IngressPacket{}
	if err := peering.Decode(res, pkt); err != nil {
		return errors.Wrap(err, "invalid response packet")
	}
	pong := pkt.Message.GetPong()
	if pong == nil {
		return errors.New("invalid response packet data")
	}

	// call the callback
	if p.onPong != nil {
		p.onPong(pkt)
	}

	return nil
}

func (p *Protocol) SendPing(to *net.UDPAddr, ping *pb.Ping) error {
	conn, err := grpc.Dial(to.String(), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	message := &pb.MessageWrapper{Message: &pb.MessageWrapper_Ping{Ping: ping}}
	req, err := peering.Encode(nil, message)
	if err != nil {
		return errors.Wrap(err, "error encoding packet")
	}

	return p.sendPing(conn, req)
}

func (p *Protocol) OnPing(f func(*peering.IngressPacket)) {
	p.onPing = f
}

func (p *Protocol) OnPong(f func(*peering.IngressPacket)) {
	p.onPong = f
}
