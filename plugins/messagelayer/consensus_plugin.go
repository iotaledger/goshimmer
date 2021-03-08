package messagelayer

import (
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/prng"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/fpc"
	votenet "github.com/iotaledger/goshimmer/packages/vote/net"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/goshimmer/packages/vote/statement"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
)

var (
	// plugin is the plugin instance of the statement plugin.
	consensusPlugin     *node.Plugin
	consensusPluginOnce sync.Once
	voter               *fpc.FPC
	voterOnce           sync.Once
	voterServer         *votenet.VoterServer
	registry            *statement.Registry
	registryOnce        sync.Once
)

// ConsensusPlugin returns the consensus plugin.
func ConsensusPlugin() *node.Plugin {
	consensusPluginOnce.Do(func() {
		consensusPlugin = node.NewPlugin("Consensus", node.Enabled, configureConsensusPlugin, runConsensusPlugin)
	})
	return consensusPlugin
}

func configureConsensusPlugin(plugin *node.Plugin) {
	configureRemoteLogger()
	configureFPC(plugin)

	// subscribe to FCOB events
	ConsensusMechanism().Events.Vote.Attach(events.NewClosure(func(id string, initOpn opinion.Opinion) {
		if err := Voter().Vote(id, vote.ConflictType, initOpn); err != nil {
			plugin.LogWarnf("FPC vote: %s", err)
		}
	}))
	ConsensusMechanism().Events.Error.Attach(events.NewClosure(func(err error) {
		plugin.LogErrorf("FCOB error: %s", err)
	}))

	// subscribe to message-layer
	Tangle().ConsensusManager.Events.MessageOpinionFormed.Attach(events.NewClosure(readStatement))
}

func runConsensusPlugin(plugin *node.Plugin) {
	runFPC(plugin)
}

// Voter returns the DRNGRoundBasedVoter instance used by the FPC plugin.
func Voter() vote.DRNGRoundBasedVoter {
	voterOnce.Do(func() {
		voter = fpc.New(OpinionGiverFunc)
	})
	return voter
}

// Registry returns the registry.
func Registry() *statement.Registry {
	registryOnce.Do(func() {
		registry = statement.NewRegistry()
	})
	return registry
}

func configureFPC(plugin *node.Plugin) {
	if FPCParameters.Listen {
		lPeer := local.GetInstance()
		_, portStr, err := net.SplitHostPort(FPCParameters.BindAddress)
		if err != nil {
			plugin.LogFatalf("FPC bind address '%s' is invalid: %s", FPCParameters.BindAddress, err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			plugin.LogFatalf("FPC bind address '%s' is invalid: %s", FPCParameters.BindAddress, err)
		}

		if err := lPeer.UpdateService(service.FPCKey, "tcp", port); err != nil {
			plugin.LogFatalf("could not update services: %v", err)
		}
	}

	Voter().Events().RoundExecuted.Attach(events.NewClosure(func(roundStats *vote.RoundStats) {
		if StatementParameters.WriteStatement {
			makeStatement(roundStats)
		}
		peersQueried := len(roundStats.QueriedOpinions)
		voteContextsCount := len(roundStats.ActiveVoteContexts)
		plugin.LogDebugf("executed round with rand %0.4f for %d vote contexts on %d peers, took %v", roundStats.RandUsed, voteContextsCount, peersQueried, roundStats.Duration)
	}))

	Voter().Events().Finalized.Attach(events.NewClosure(ConsensusMechanism().ProcessVote))
	Voter().Events().Finalized.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		if ev.Ctx.Type == vote.ConflictType {
			plugin.LogInfof("FPC finalized for transaction with id '%s' - final opinion: '%s'", ev.ID, ev.Opinion)
		}
	}))

	Voter().Events().Failed.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		if ev.Ctx.Type == vote.ConflictType {
			plugin.LogWarnf("FPC failed for transaction with id '%s' - last opinion: '%s'", ev.ID, ev.Opinion)
		}
	}))

}

func runFPC(plugin *node.Plugin) {
	const ServerWorkerName = "FPCVoterServer"

	if FPCParameters.Listen {
		if err := daemon.BackgroundWorker(ServerWorkerName, func(shutdownSignal <-chan struct{}) {
			stopped := make(chan struct{})
			bindAddr := FPCParameters.BindAddress
			voterServer = votenet.New(Voter(), OpinionRetriever, bindAddr,
				metrics.Events().FPCInboundBytes,
				metrics.Events().FPCOutboundBytes,
				metrics.Events().QueryReceived,
			)

			go func() {
				plugin.LogInfof("%s started, bind-address=%s", ServerWorkerName, bindAddr)
				if err := voterServer.Run(); err != nil {
					plugin.LogErrorf("Error serving: %s", err)
				}
				close(stopped)
			}()

			// stop if we are shutting down or the server could not be started
			select {
			case <-shutdownSignal:
			case <-stopped:
			}

			plugin.LogInfof("Stopping %s ...", ServerWorkerName)
			voterServer.Shutdown()
			plugin.LogInfof("Stopping %s ... done", ServerWorkerName)
		}, shutdown.PriorityFPC); err != nil {
			plugin.Panicf("Failed to start as daemon: %s", err)
		}
	}

	if err := daemon.BackgroundWorker("FPCRoundsInitiator", func(shutdownSignal <-chan struct{}) {
		plugin.LogInfof("Started FPC round initiator")
		defer plugin.LogInfof("Stopped FPC round initiator")
		unixTsPRNG := prng.NewUnixTimestampPRNG(FPCParameters.RoundInterval)
		unixTsPRNG.Start()
		defer unixTsPRNG.Stop()
	exit:
		for {
			select {
			case r := <-unixTsPRNG.C():
				if err := voter.Round(r); err != nil {
					plugin.LogWarnf("unable to execute FPC round: %s", err)
				}
			case <-shutdownSignal:
				break exit
			}
		}
	}, shutdown.PriorityFPC); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}

	if err := daemon.BackgroundWorker("StatementCleaner", func(shutdownSignal <-chan struct{}) {
		plugin.LogInfof("Started Statement Cleaner")
		defer plugin.LogInfof("Stopped Statement Cleaner")
		ticker := time.NewTicker(time.Duration(StatementParameters.CleanInterval) * time.Minute)
		defer ticker.Stop()
	exit:
		for {
			select {
			case <-ticker.C:
				Registry().Clean(time.Duration(StatementParameters.DeleteAfter) * time.Minute)
			case <-shutdownSignal:
				break exit
			}
		}
	}, shutdown.PriorityFPC); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}
}
