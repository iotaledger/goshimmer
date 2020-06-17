package net

import (
	"context"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/hive.go/events"
	"google.golang.org/grpc"
)

// OpinionRetriever retrieves the opinion for the given ID.
// If there's no opinion, the function should return Unknown.
type OpinionRetriever func(id string) vote.Opinion

// New creates a new VoterServer.
func New(voter vote.Voter, opnRetriever OpinionRetriever, bindAddr string, netEvents ...*events.Event) *VoterServer {
	vs := &VoterServer{
		voter:        voter,
		opnRetriever: opnRetriever,
		bindAddr:     bindAddr,
	}
	if netEvents == nil && len(netEvents) < 2 {
		return vs
	}

	vs.netEventRX = netEvents[0]
	vs.netEventTX = netEvents[1]

	return vs
}

// VoterServer is a server which responds to opinion queries.
type VoterServer struct {
	voter        vote.Voter
	opnRetriever OpinionRetriever
	bindAddr     string
	grpcServer   *grpc.Server
	netEventRX   *events.Event
	netEventTX   *events.Event
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

	if vs.netEventRX != nil {
		vs.netEventRX.Trigger(proto.Size(req))
	}
	if vs.netEventTX != nil {
		vs.netEventTX.Trigger(proto.Size(reply))
	}

	metrics.Events().QueryReceived.Trigger(&metrics.QueryReceivedEvent{OpinionCount: len(req.Id)})
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
