package client

import (
	"context"
	"io"
	"log"

	pb "github.com/iotaledger/goshimmer/plugins/fpc/prng/random"
	"google.golang.org/grpc"
)

// Tick defines the pair to store a new
// round index and random
type Tick struct {
	Index uint64  // Round index
	Value float64 // Random [0,1)
}

// Ticker defines a channel of Tick
type Ticker struct {
	C chan *Tick
}

// receiveRandoms fills the ticker channel with new random
func (t *Ticker) receiveRandoms(client pb.RandomGeneratorClient) {
	ctx := context.Background() // WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()
	stream, err := client.Subscribe(ctx, &pb.Void{})
	if err != nil {
		//TODO: notify error to the fpc plugin
		log.Fatalf("%v.Subscribe(_) = _, %v", client, err)
	}
	for {
		random, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			//TODO: notify error to the fpc plugin
			log.Fatalf("%v.Recv(_) = _, %v", stream, err)
		}
		t.C <- &Tick{
			Index: uint64(random.GetIndex()),
			Value: random.GetValue(),
		}
	}
}

// NewTicker returns a pointer to a new Ticker
func NewTicker() *Ticker {
	return &Ticker{
		C: make(chan *Tick),
	}
}

// Connect opens a new gRPC stream from this client
// to the centralized RNG serverAddr and fill
// the ticker channel with new random
func (t *Ticker) Connect(serverAddr string) {
	go func() {
		var opts []grpc.DialOption

		opts = append(opts, grpc.WithInsecure())

		conn, err := grpc.Dial(serverAddr, opts...)
		if err != nil {
			log.Fatalf("fail to dial: %v", err)
		}
		defer conn.Close()
		client := pb.NewRandomGeneratorClient(conn)

		t.receiveRandoms(client)
	}()
}
