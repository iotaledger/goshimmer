package server

import (
	"context"
	"flag"
	"net"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/ternary"
	autop "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
	pb "github.com/iotaledger/goshimmer/plugins/fpc/network/query"
	"github.com/iotaledger/goshimmer/plugins/tangle"
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
// Currently, we only look for opinions by calling fpc.GetInterimOpinion
func (s *queryServer) GetOpinion(ctx context.Context, req *pb.QueryRequest) (*pb.QueryReply, error) {
	opinions, missingTxs := s.fpc.GetInterimOpinion(req.TxHash...)
	reply := &pb.QueryReply{
		Opinion: opinions,
	}
	for _, missingTx := range missingTxs {
		txMetadata, err := tangle.GetTransactionMetadata(ternary.Trytes(req.TxHash[missingTx]))
		if err == nil {
			reply.Opinion[missingTx] = txMetadata.GetLiked()
		}
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
