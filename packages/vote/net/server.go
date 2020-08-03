package net

import (
	"context"
	"net"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/hive.go/events"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// OpinionRetriever retrieves the opinion for the given ID.
// If there's no opinion, the function should return Unknown.
type OpinionRetriever func(id string) vote.Opinion

// New creates a new VoterServer.
func New(voter vote.Voter, opnRetriever OpinionRetriever, bindAddr string, netRxEvent, netTxEvent, queryReceivedEvent *events.Event) *VoterServer {
	return &VoterServer{
		voter:              voter,
		opnRetriever:       opnRetriever,
		bindAddr:           bindAddr,
		grpcServer:         grpc.NewServer(),
		netRxEvent:         netRxEvent,
		netTxEvent:         netTxEvent,
		queryReceivedEvent: queryReceivedEvent,
	}
}

// VoterServer is a server which responds to opinion queries.
type VoterServer struct {
	voter              vote.Voter
	opnRetriever       OpinionRetriever
	bindAddr           string
	grpcServer         *grpc.Server
	netRxEvent         *events.Event
	netTxEvent         *events.Event
	queryReceivedEvent *events.Event
}

// Opinion replies the query request with an opinion and triggers the events.
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

	if vs.netRxEvent != nil {
		vs.netRxEvent.Trigger(uint64(proto.Size(req)))
	}
	if vs.netTxEvent != nil {
		vs.netTxEvent.Trigger(uint64(proto.Size(reply)))
	}
	if vs.queryReceivedEvent != nil {
		vs.queryReceivedEvent.Trigger(&metrics.QueryReceivedEvent{OpinionCount: len(req.Id)})
	}

	return reply, nil
}

// Run starts the voting server.
func (vs *VoterServer) Run() error {
	listener, err := net.Listen("tcp", vs.bindAddr)
	if err != nil {
		return err
	}

	RegisterVoterQueryServer(vs.grpcServer, vs)
	return vs.grpcServer.Serve(listener)
}

// Shutdown shutdowns the voting server.
func (vs *VoterServer) Shutdown() {
	vs.grpcServer.GracefulStop()
}
