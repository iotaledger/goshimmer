package net

import (
	"context"
	"net"

	"github.com/iotaledger/goshimmer/packages/vote"
	"google.golang.org/grpc"
)

// OpinionRetriever retrieves the opinion for the given ID.
// If there's no opinion, the function should return Unknown.
type OpinionRetriever func(id string) vote.Opinion

// New creates a new VoterServer.
func New(voter vote.Voter, opnRetriever OpinionRetriever, bindAddr string) *VoterServer {
	vs := &VoterServer{
		voter:        voter,
		opnRetriever: opnRetriever,
		bindAddr:     bindAddr,
	}
	return vs
}

// VoterServer is a server which responds to opinion queries.
type VoterServer struct {
	voter        vote.Voter
	opnRetriever OpinionRetriever
	bindAddr     string
	grpcServer   *grpc.Server
}

func (vs *VoterServer) Opinion(ctx context.Context, req *QueryRequest) (*QueryReply, error) {
	reply := &QueryReply{
		Opinion: make([]int32, len(req.Id)),
	}
	for i, id := range req.Id {
		// check whether there's an ongoing vote
		opinion, err := vs.voter.IntermediateOpinion(id)
		if err == nil {
			reply.Opinion[i] = int32(opinion)
			continue
		}
		reply.Opinion[i] = int32(vs.opnRetriever(id))
	}

	return reply, nil
}

func (vs *VoterServer) Run() error {
	listener, err := net.Listen("tcp", vs.bindAddr)
	if err != nil {
		return err
	}

	vs.grpcServer = grpc.NewServer()
	RegisterVoterQueryServer(vs.grpcServer, vs)

	return vs.grpcServer.Serve(listener)
}

func (vs *VoterServer) Shutdown() {
	vs.grpcServer.GracefulStop()
}
