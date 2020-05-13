package fpc

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/iotaledger/goshimmer/packages/prng"
	"github.com/iotaledger/hive.go/events"
	"google.golang.org/grpc"

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
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the FPC plugin.
const PluginName = "FPC"

var (
	// Plugin is the plugin instance of the FPC plugin.
	Plugin               = node.NewPlugin(PluginName, node.Enabled, configure, run)
	log                  *logger.Logger
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

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	lPeer := local.GetInstance()

	bindAddr := config.Node.GetString(CfgFPCBindAddress)
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

	voter.Events().RoundExecuted.Attach(events.NewClosure(func(roundStats *vote.RoundStats) {
		peersQueried := len(roundStats.QueriedOpinions)
		voteContextsCount := len(roundStats.ActiveVoteContexts)
		log.Infof("executed round with rand %0.4f for %d vote contexts on %d peers, took %v", roundStats.RandUsed, voteContextsCount, peersQueried, roundStats.Duration)
	}))
}

func run(_ *node.Plugin) {

	daemon.BackgroundWorker("FPCVoterServer", func(shutdownSignal <-chan struct{}) {
		voterServer = votenet.New(Voter(), func(id string) vote.Opinion {
			// TODO: replace with persistence layer call
			return vote.Unknown
		}, config.Node.GetString(CfgFPCBindAddress))

		go func() {
			if err := voterServer.Run(); err != nil {
				log.Error(err)
			}
		}()

		log.Infof("Started vote server on %s", config.Node.GetString(CfgFPCBindAddress))
		<-shutdownSignal
		voterServer.Shutdown()
		log.Info("Stopped vote server")
	}, shutdown.PriorityFPC)

	daemon.BackgroundWorker("FPCRoundsInitiator", func(shutdownSignal <-chan struct{}) {
		log.Infof("Started FPC round initiator")
		unixTsPRNG := prng.NewUnixTimestampPRNG(roundIntervalSeconds)
		defer unixTsPRNG.Stop()
	exit:
		for {
			select {
			case r := <-unixTsPRNG.C():
				if err := voter.Round(r); err != nil {
					log.Errorf("unable to execute FPC round: %s", err)
				}
			case <-shutdownSignal:
				break exit
			}
		}
		log.Infof("Stopped FPC round initiator")
	}, shutdown.PriorityFPC)
}

// PeerOpinionGiver implements the OpinionGiver interface based on a peer.
type PeerOpinionGiver struct {
	p *peer.Peer
}

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
	reply, err := client.Opinion(ctx, &votenet.QueryRequest{Id: ids})
	if err != nil {
		return nil, fmt.Errorf("unable to query opinions: %w", err)
	}

	// convert int32s in reply to opinions
	opinions := make(vote.Opinions, len(reply.Opinion))
	for i, intOpn := range reply.Opinion {
		opinions[i] = vote.ConvertInt32Opinion(intOpn)
	}

	return opinions, nil
}

func (pog *PeerOpinionGiver) ID() string {
	return pog.p.ID().String()
}
