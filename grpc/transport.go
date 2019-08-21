package grpc

import (
	context "context"
	"log"
	"net"

	"github.com/pkg/errors"
	pb "github.com/wollac/autopeering/proto"
	"google.golang.org/grpc"
)

// Errors
var (
	errCancelled = errors.New("cancelled")
)

type Transport struct {
	server *grpc.Server
	addr   net.Addr

	ch chan *pb.Packet
}

func Start(address string) *Transport {
	t := &Transport{
		ch: make(chan *pb.Packet),
	}

	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	peeringServer := &server{t: t}
	RegisterPeeringServer(grpcServer, peeringServer)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	t.server = grpcServer
	t.addr = lis.Addr()

	return t
}

func (t *Transport) Close() {
	close(t.ch)
	t.server.Stop()
	t.server = nil
}

func (t *Transport) ReadFrom() (*pb.Packet, net.Addr, error) {
	res := <-t.ch
	return res, t.addr, nil
}

func (t *Transport) WriteTo(req *pb.Packet, to net.Addr) error {
	conn, err := grpc.Dial(to.String(), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	client := NewPeeringClient(conn)
	_, err = client.Send(context.Background(), req)
	if err != nil {
		return errors.Wrap(err, "error encoding packet")
	}
	return nil
}

type server struct {
	t *Transport
}

func (s *server) Send(ctx context.Context, req *pb.Packet) (*Empty, error) {
	select {
	case s.t.ch <- req:
		return &Empty{}, nil
	case <-ctx.Done():
		return nil, errCancelled
	}
}
