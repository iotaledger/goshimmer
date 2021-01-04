package consensus

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/vote"
	votenet "github.com/iotaledger/goshimmer/packages/vote/net"
	"github.com/iotaledger/goshimmer/packages/vote/statement"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/identity"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// region OpinionGivers /////////////////////////////////////////////////////////////////////////////////////////////////////

// OpinionGiver is a wrapper for both statements and peers.
type OpinionGiver struct {
	id   identity.ID
	view *statement.View
	pog  *PeerOpinionGiver
}

// OpinionGivers is a map of OpinionGiver.
type OpinionGivers map[identity.ID]OpinionGiver

// Query retrievs the opinions about the given conflicts and timestamps.
func (o *OpinionGiver) Query(ctx context.Context, conflictIDs []string, timestampIDs []string) (opinions vote.Opinions, err error) {
	for i := 0; i < waitForStatement; i++ {
		if o.view != nil {
			opinions, err = o.view.Query(ctx, conflictIDs, timestampIDs)
			if err == nil {
				return opinions, nil
			}
		}
		time.Sleep(time.Second)
	}

	return o.pog.Query(ctx, conflictIDs, timestampIDs)
}

// ID returns the identifier of the underlying Peer.
func (o *OpinionGiver) ID() identity.ID {
	return o.id
}

// OpinionGiverFunc returns a slice of opinion givers.
func OpinionGiverFunc() (givers []vote.OpinionGiver, err error) {
	opinionGiversMap := make(map[identity.ID]*OpinionGiver)
	opinionGivers := make([]vote.OpinionGiver, 0)

	for _, v := range Registry().NodesView() {
		opinionGiversMap[v.ID()] = &OpinionGiver{
			id:   v.ID(),
			view: v,
		}
	}

	for _, p := range autopeering.Discovery().GetVerifiedPeers() {
		fpcService := p.Services().Get(service.FPCKey)
		if fpcService == nil {
			continue
		}
		if _, ok := opinionGiversMap[p.ID()]; !ok {
			opinionGiversMap[p.ID()] = &OpinionGiver{
				id:   p.ID(),
				view: nil,
			}
		}
		opinionGiversMap[p.ID()].pog = &PeerOpinionGiver{p: p}
	}

	for _, v := range opinionGiversMap {
		opinionGivers = append(opinionGivers, v)
	}

	return opinionGivers, nil
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////////

// region PeerOpinionGiver /////////////////////////////////////////////////////////////////////////////////////////////////////

// PeerOpinionGiver implements the OpinionGiver interface based on a peer.
type PeerOpinionGiver struct {
	p *peer.Peer
}

// Query queries another node for its opinion.
func (pog *PeerOpinionGiver) Query(ctx context.Context, conflictIDs []string, timestampIDs []string) (vote.Opinions, error) {
	if pog == nil {
		return nil, fmt.Errorf("unable to query opinions, PeerOpinionGiver is nil")
	}

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())

	// connect to the FPC service
	conn, err := grpc.Dial(pog.Address(), opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to FPC service: %w", err)
	}
	defer conn.Close()

	client := votenet.NewVoterQueryClient(conn)
	query := &votenet.QueryRequest{ConflictIDs: conflictIDs, TimestampIDs: timestampIDs}
	reply, err := client.Opinion(ctx, query)
	if err != nil {
		metrics.Events().QueryReplyError.Trigger(&metrics.QueryReplyErrorEvent{
			ID:           pog.p.ID().String(),
			OpinionCount: len(conflictIDs) + len(timestampIDs),
		})
		return nil, fmt.Errorf("unable to query opinions: %w", err)
	}

	metrics.Events().FPCInboundBytes.Trigger(uint64(proto.Size(reply)))
	metrics.Events().FPCOutboundBytes.Trigger(uint64(proto.Size(query)))

	// convert int32s in reply to opinions
	opinions := make(vote.Opinions, len(reply.Opinion))
	for i, intOpn := range reply.Opinion {
		opinions[i] = vote.ConvertInt32Opinion(intOpn)
	}

	return opinions, nil
}

// ID returns the identifier of the underlying Peer.
func (pog *PeerOpinionGiver) ID() identity.ID {
	return pog.p.ID()
}

// Address returns the FPC address of the underlying Peer.
func (pog *PeerOpinionGiver) Address() string {
	fpcServicePort := pog.p.Services().Get(service.FPCKey).Port()
	return net.JoinHostPort(pog.p.IP().String(), strconv.Itoa(fpcServicePort))
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////////
