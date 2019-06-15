package server

import (
	"context"
	"flag"
	"net"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/node"
	autop "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
	"github.com/iotaledger/goshimmer/plugins/fpc/network/query"
	pb "github.com/iotaledger/goshimmer/plugins/fpc/network/query"
	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 10000, "The server port")
)

// queryServer defines the struct of an FPC query server
type queryServer struct {
	fpc *fpc.Instance
}

func newServer(fpc *fpc.Instance) *queryServer {
	return &queryServer{
		fpc: fpc,
	}
}

// GetOpinion returns the opinions of the given txs.
func (s *queryServer) GetOpinion(ctx context.Context, req *pb.QueryRequest) (*pb.QueryReply, error) {
	opinions := make([]pb.QueryReply_Opinion, len(req.GetTxHash()))
	// converting QueryRequest strings to fpc.ID
	requestedIDs := make([]fpc.ID, len(req.GetTxHash()))
	for i, txHash := range req.GetTxHash() {
		requestedIDs[i] = fpc.ID(txHash)
	}
	myOpinions := s.fpc.GetInterimOpinion(requestedIDs...)
	// TODO: look for tx opinion into tangle db/cache
	// also include the check with the solidTimestamp
	for i := range req.GetTxHash() {
		opinions[i] = query.QueryReply_Opinion(myOpinions[i])
	}

	reply := &pb.QueryReply{
		Opinion: opinions,
	}
	return reply, nil
}

// run starts a new server for replying to incoming queries
func run(address, port string, fpc *fpc.Instance) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", address+":"+port)
	if err != nil {
		return nil, err
	}
	var opts []grpc.ServerOption

	server := newServer(fpc)

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterFPCQueryServer(grpcServer, server)

	// Starts as a goroutine so that it does not block
	// and can return the grpcServer pointer to the caller
	go grpcServer.Serve(lis)
	return grpcServer, err
}

func RunServer(plugin *node.Plugin, fpc *fpc.Instance) {
	plugin.LogInfo("Starting TCP Server (port " + strconv.Itoa(*autop.PORT.Value+2000) + ") ...")

	daemon.BackgroundWorker(func() {
		plugin.LogSuccess("Starting TCP Server (port " + strconv.Itoa(*autop.PORT.Value+2000) + ") ... done")

		server, _ := run("0.0.0.0", strconv.Itoa(*autop.PORT.Value+2000), fpc)

		// Waits until receives a shutdown signal
		select {
		case <-daemon.ShutdownSignal:
			plugin.LogInfo("Stopping TCP Server ...")
			server.GracefulStop()
		}

		plugin.LogSuccess("Stopping TCP Server ... done")
	})
}
