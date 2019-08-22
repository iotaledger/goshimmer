package transport

import (
	"context"
	"io"
	"log"
	"net"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	pb "github.com/wollac/autopeering/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// TransportGRPC offers gRPC based transfers.
type TransportGRPC struct {
	srv       *grpc.Server
	localAddr string
	options   []grpc.DialOption
	ch        chan transfer

	wg        sync.WaitGroup
	closeOnce sync.Once
	closing   chan struct{}
}

// GRPC initiates gRPC based packet transfer. The gRPC server is started for
// the given listener.
func GRPC(lis net.Listener) *TransportGRPC {
	grpcServer := grpc.NewServer()
	t := &TransportGRPC{
		srv:       grpcServer,
		localAddr: lis.Addr().String(),
		options:   []grpc.DialOption{},
		ch:        make(chan transfer, 1),
		closing:   make(chan struct{}),
	}

	pb.RegisterPeeringServer(grpcServer, &server{t})

	starting := make(chan bool)
	t.wg.Add(1)
	go t.serve(lis, starting)

	// it is not possible to wait until the server is up, but we should at
	// least wait until the goroutine is executed, to avoid scheduler issues
	<-starting

	return t
}

// Runs the gRPC server in background.
func (t *TransportGRPC) serve(lis net.Listener, starting chan<- bool) {
	defer t.wg.Done()
	defer lis.Close()

	starting <- true
	if err := t.srv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// SetDialOptions sets the options used for subsequent dial calls.
// Previously set dial options will be overridden.
func (t *TransportGRPC) SetDialOptions(opts ...grpc.DialOption) {
	t.options = append([]grpc.DialOption{}, opts...)
}

// ReadFrom implements the Transport ReadFrom method.
func (t *TransportGRPC) ReadFrom() (*pb.Packet, string, error) {
	select {
	case res := <-t.ch:
		return res.pkt, res.addr, nil
	case <-t.closing:
		return nil, "", io.EOF
	}
}

// WriteTo implements the Transport WriteTo method.
func (t *TransportGRPC) WriteTo(pkt *pb.Packet, address string) error {
	conn, err := grpc.Dial(address, t.options...)
	if err != nil {
		return errors.Wrap(err, "failed to dial")
	}
	defer conn.Close()

	if _, err := pb.NewPeeringClient(conn).Send(context.Background(), pkt); err != nil {
		return errors.Wrap(err, "error encoding packet")
	}
	return nil
}

// Close closes the transport layer.
func (t *TransportGRPC) Close() {
	t.closeOnce.Do(func() {
		close(t.closing)
		t.srv.Stop()
		t.wg.Wait()
		t.srv = nil
	})
}

// LocalAddr returns the local network address.
func (t *TransportGRPC) LocalAddr() string {
	return t.localAddr
}

type server struct {
	t *TransportGRPC
}

func (s *server) Send(ctx context.Context, pkt *pb.Packet) (*empty.Empty, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errPeer
	}

	select {
	case s.t.ch <- transfer{pkt: pkt, addr: peer.Addr.String()}:
		return new(empty.Empty), nil
	case <-s.t.closing:
		return nil, errClosed
	case <-ctx.Done():
		return nil, errClosed
	}
}
