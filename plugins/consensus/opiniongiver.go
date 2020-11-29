package consensus

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/vote"
	votenet "github.com/iotaledger/goshimmer/packages/vote/net"
	"github.com/iotaledger/goshimmer/packages/vote/statement"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"google.golang.org/grpc"
)

// OpinionGiver is a wrapper for both statements and peers.
type OpinionGiver struct {
	id   string
	view *statement.View
	pog  *PeerOpinionGiver
}

// OpinionGivers is a map of OpinionGiver.
type OpinionGivers map[string]OpinionGiver

// Query retrievs the opinions about the given conflicts and timestamps.
func (o *OpinionGiver) Query(ctx context.Context, conflictIDs []string, timestampIDs []string) (opinions vote.Opinions, err error) {
	for i := 0; i < waitForStatement; i++ {
		opinions, err = o.view.Query(ctx, conflictIDs, timestampIDs)
		if err == nil {
			return opinions, nil
		}
		time.Sleep(time.Second)
	}

	opinions, err = o.pog.Query(ctx, conflictIDs, timestampIDs)
	if err != nil {
		return nil, err
	}

	return opinions, nil
}

// ID returns a string representation of the identifier of the underlying Peer.
func (o *OpinionGiver) ID() string {
	return o.id
}

// OpinionGiverFunc returns a slice of opinion givers.
func OpinionGiverFunc() (givers []vote.OpinionGiver, err error) {
	opinionGiversMap := make(map[string]*OpinionGiver)
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
		if _, ok := opinionGiversMap[p.ID().String()]; !ok {
			opinionGiversMap[p.ID().String()] = &OpinionGiver{
				id: p.ID().String(),
			}
		}
		opinionGiversMap[p.ID().String()].pog = &PeerOpinionGiver{p: p}
	}

	for _, v := range opinionGiversMap {
		opinionGivers = append(opinionGivers, &OpinionGiver{
			id:   v.id,
			view: v.view,
			pog:  v.pog,
		})
	}

	return opinionGivers, nil
}

// PeerOpinionGiver implements the OpinionGiver interface based on a peer.
type PeerOpinionGiver struct {
	p *peer.Peer
}

// Query queries another node for its opinion.
func (pog *PeerOpinionGiver) Query(ctx context.Context, conflictIDs []string, timestampIDs []string) (vote.Opinions, error) {
	fpcServicePort := pog.p.Services().Get(service.FPCKey).Port()
	fpcAddr := net.JoinHostPort(pog.p.IP().String(), strconv.Itoa(fpcServicePort))

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())

	// connect to the FPC service
	conn, err := grpc.Dial(fpcAddr, opts...)
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

// ID returns a string representation of the identifier of the underlying Peer.
func (pog *PeerOpinionGiver) ID() string {
	return pog.p.ID().String()
}

// OpinionRetriever returns the current opinion of the given id.
func OpinionRetriever(id string, objectType vote.ObjectType) vote.Opinion {
	switch objectType {
	case vote.TimestampType:
		// TODO: implement
		return vote.Like
	default: // conflict type
		branchID, err := branchmanager.BranchIDFromBase58(id)
		if err != nil {
			log.Errorf("received invalid vote request for branch '%s'", id)

			return vote.Unknown
		}

		cachedBranch := valuetransfers.Tangle().BranchManager().Branch(branchID)
		defer cachedBranch.Release()

		branch := cachedBranch.Unwrap()
		if branch == nil {
			return vote.Unknown
		}

		if !branch.Preferred() {
			return vote.Dislike
		}

		return vote.Like
	}
}
