package valuetransfers

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/prng"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/fpc"
	votenet "github.com/iotaledger/goshimmer/packages/vote/net"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	flag "github.com/spf13/pflag"
	"google.golang.org/grpc"
)

const (
	// FpcPluginName contains the human readable name of the plugin.
	FpcPluginName = "FPC"

	// CfgFPCQuerySampleSize defines how many nodes will be queried each round.
	CfgFPCQuerySampleSize = "fpc.querySampleSize"

	// CfgFPCRoundInterval defines how long a round lasts (in seconds)
	CfgFPCRoundInterval = "fpc.roundInterval"

	// CfgFPCBindAddress defines on which address the FPC service should listen.
	CfgFPCBindAddress = "fpc.bindAddress"
)

func init() {
	flag.Int(CfgFPCQuerySampleSize, 21, "Size of the voting quorum (k)")
	flag.Int(CfgFPCRoundInterval, 5, "FPC round interval [s]")
	flag.String(CfgFPCBindAddress, "0.0.0.0:10895", "the bind address on which the FPC vote server binds to")
}

var (
	voter                *fpc.FPC
	voterOnce            sync.Once
	voterServer          *votenet.VoterServer
	roundIntervalSeconds int64 = 5
)

// Voter returns the DRNGRoundBasedVoter instance used by the FPC plugin.
func Voter() vote.DRNGRoundBasedVoter {
	voterOnce.Do(func() {
		// create a function which gets OpinionGivers
		opinionGiverFunc := func() (givers []vote.OpinionGiver, err error) {
			opinionGivers := make([]vote.OpinionGiver, 0)
			for _, p := range autopeering.Discovery().GetVerifiedPeers() {
				fpcService := p.Services().Get(service.FPCKey)
				if fpcService == nil {
					continue
				}
				// TODO: maybe cache the PeerOpinionGiver instead of creating a new one every time
				opinionGivers = append(opinionGivers, &PeerOpinionGiver{p: p})
			}
			return opinionGivers, nil
		}
		voter = fpc.New(opinionGiverFunc)
	})
	return voter
}

func configureFPC() {
	log = logger.NewLogger(FpcPluginName)
	lPeer := local.GetInstance()

	bindAddr := config.Node().GetString(CfgFPCBindAddress)
	_, portStr, err := net.SplitHostPort(bindAddr)
	if err != nil {
		log.Fatalf("FPC bind address '%s' is invalid: %s", bindAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("FPC bind address '%s' is invalid: %s", bindAddr, err)
	}

	if err := lPeer.UpdateService(service.FPCKey, "tcp", port); err != nil {
		log.Fatalf("could not update services: %v", err)
	}

	Voter().Events().RoundExecuted.Attach(events.NewClosure(func(roundStats *vote.RoundStats) {
		peersQueried := len(roundStats.QueriedOpinions)
		voteContextsCount := len(roundStats.ActiveVoteContexts)
		log.Debugf("executed round with rand %0.4f for %d vote contexts on %d peers, took %v", roundStats.RandUsed, voteContextsCount, peersQueried, roundStats.Duration)
	}))
}

func runFPC() {
	const ServerWorkerName = "FPCVoterServer"
	if err := daemon.BackgroundWorker(ServerWorkerName, func(shutdownSignal <-chan struct{}) {
		stopped := make(chan struct{})
		bindAddr := config.Node().GetString(CfgFPCBindAddress)
		voterServer = votenet.New(Voter(), func(id string) vote.Opinion {
			branchID, err := branchmanager.BranchIDFromBase58(id)
			if err != nil {
				log.Errorf("received invalid vote request for branch '%s'", id)

				return vote.Unknown
			}

			cachedBranch := _tangle.BranchManager().Branch(branchID)
			defer cachedBranch.Release()

			branch := cachedBranch.Unwrap()
			if branch == nil {
				return vote.Unknown
			}

			if !branch.Preferred() {
				return vote.Dislike
			}

			return vote.Like
		}, bindAddr,
			metrics.Events().FPCInboundBytes,
			metrics.Events().FPCOutboundBytes,
			metrics.Events().QueryReceived,
		)

		go func() {
			log.Infof("%s started, bind-address=%s", ServerWorkerName, bindAddr)
			if err := voterServer.Run(); err != nil {
				log.Errorf("Error serving: %s", err)
			}
			close(stopped)
		}()

		// stop if we are shutting down or the server could not be started
		select {
		case <-shutdownSignal:
		case <-stopped:
		}

		log.Infof("Stopping %s ...", ServerWorkerName)
		voterServer.Shutdown()
		log.Infof("Stopping %s ... done", ServerWorkerName)
	}, shutdown.PriorityFPC); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}

	if err := daemon.BackgroundWorker("FPCRoundsInitiator", func(shutdownSignal <-chan struct{}) {
		log.Infof("Started FPC round initiator")
		defer log.Infof("Stopped FPC round initiator")
		unixTsPRNG := prng.NewUnixTimestampPRNG(roundIntervalSeconds)
		unixTsPRNG.Start()
		defer unixTsPRNG.Stop()
	exit:
		for {
			select {
			case r := <-unixTsPRNG.C():
				if err := voter.Round(r); err != nil {
					log.Warnf("unable to execute FPC round: %s", err)
				}
			case <-shutdownSignal:
				break exit
			}
		}
	}, shutdown.PriorityFPC); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// PeerOpinionGiver implements the OpinionGiver interface based on a peer.
type PeerOpinionGiver struct {
	p *peer.Peer
}

// Query queries another node for its opinion.
func (pog *PeerOpinionGiver) Query(ctx context.Context, ids []string) (vote.Opinions, error) {
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
	query := &votenet.QueryRequest{Id: ids}
	reply, err := client.Opinion(ctx, query)
	if err != nil {
		metrics.Events().QueryReplyError.Trigger(&metrics.QueryReplyErrorEvent{
			ID:           pog.p.ID().String(),
			OpinionCount: len(ids),
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
