package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"

	guuid "github.com/google/uuid"
	pb "github.com/iotaledger/goshimmer/plugins/fpc/prng/random"
)

var (
	port     = flag.Int("port", 10000, "The server port")
	interval = flag.Int("interval", 2, "The time interval of random number generation (s)")
)

type prngServer struct {
	rc             sync.RWMutex
	registeredChan map[string]chan *pb.Random
}

func newServer() *prngServer {
	return &prngServer{registeredChan: make(map[string]chan *pb.Random)}
}

func (s *prngServer) registerNewChan(id string, c chan *pb.Random) {
	s.rc.Lock()
	defer s.rc.Unlock()
	s.registeredChan[id] = c
	fmt.Println("Channel registered:", len(s.registeredChan))
}

func (s *prngServer) unsubsribeChan(id string) {
	s.rc.Lock()
	defer s.rc.Unlock()
	delete(s.registeredChan, id)
}

func (s *prngServer) serve(newRandom *pb.Random) {
	s.rc.RLock()
	defer s.rc.RUnlock()
	for _, c := range s.registeredChan {
		c <- newRandom
	}
}

func (s *prngServer) run() {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	i := int64(0)

	for {
		i = (i + 1) % math.MaxInt64
		newRandom := &pb.Random{
			Index: i,
			Value: rand.Float64(),
		}
		fmt.Println(newRandom.GetIndex(), newRandom.GetValue())
		s.serve(newRandom)
		time.Sleep(time.Duration(*interval) * time.Second)
	}
}

// Subscribe creates a subscription for a new client.
func (s *prngServer) Subscribe(void *pb.Void, stream pb.RandomGenerator_SubscribeServer) error {
	c := make(chan *pb.Random)
	id := guuid.New().String()
	fmt.Println("New Connection:", id)
	s.registerNewChan(id, c)
	for {
		select {
		case random := <-c:
			if err := stream.Send(random); err != nil {
				fmt.Printf("%v.Send(_) = _, %v\n", stream, err)
				// connection dropped?
				// if so, remove it
				s.unsubsribeChan(id)
				close(c)
				return err
			}
		}
	}
}

func main() {
	flag.Parse()
	fmt.Println("Starting on port:", *port)
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption

	s := newServer()
	go s.run()

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterRandomGeneratorServer(grpcServer, s)
	grpcServer.Serve(lis)
}
