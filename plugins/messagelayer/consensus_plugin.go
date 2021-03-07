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
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

const (
	// ConsensusPluginName contains the human readable name of the plugin.
	ConsensusPluginName = "Consensus"

	// CfgWaitForStatement is the time in seconds for which the node wait for receiving the new statement.
	CfgWaitForStatement = "statement.waitForStatement"

	// CfgManaThreshold defines the Mana threshold to accept/write a statement.
	CfgManaThreshold = "statement.manaThreshold"

	// CfgCleanInterval defines the time interval [in minutes] for cleaning the statement registry.
	CfgCleanInterval = "statement.cleanInterval"

	// CfgDeleteAfter defines the time [in minutes] after which older statements are deleted from the registry.
	CfgDeleteAfter = "statement.deleteAfter"
	// CfgWriteStatement defines if the node should write statements.
	CfgWriteStatement = "statement.writeStatement"
)

func init() {
	flag.Bool(CfgWriteStatement, false, "if the node should make statements")
	flag.Int(CfgWaitForStatement, 5, "the time in seconds for which the node wait for receiving the new statement")
	flag.Float64(CfgManaThreshold, 1., "Mana threshold to accept/write a statement")
	flag.Int(CfgCleanInterval, 5, "the time in minutes after which the node cleans the statement registry")
	flag.Int(CfgDeleteAfter, 5, "the time in minutes after which older statements are deleted from the registry")
}

var (
	// plugin is the plugin instance of the statement plugin.
	consensusPlugin     *node.Plugin
	consensusPluginOnce sync.Once
	voter               *fpc.FPC
	voterOnce           sync.Once
	voterServer         *votenet.VoterServer
	consensusPluginLog  *logger.Logger
	registry            *statement.Registry
	registryOnce        sync.Once
	waitForStatement    int
	cleanInterval       int
	deleteAfter         int
	writeStatement      bool
)

// ConsensusPlugin returns the consensus plugin.
func ConsensusPlugin() *node.Plugin {
	consensusPluginOnce.Do(func() {
		consensusPlugin = node.NewPlugin(ConsensusPluginName, node.Enabled, configureConsensusPlugin, runConsensusPlugin)
	})
	return consensusPlugin
}

func configureConsensusPlugin(*node.Plugin) {
	consensusPluginLog = logger.NewLogger(ConsensusPluginName)

	configureRemoteLogger()

	waitForStatement = config.Node().Int(CfgWaitForStatement)
	cleanInterval = config.Node().Int(CfgCleanInterval)
	deleteAfter = config.Node().Int(CfgDeleteAfter)
	writeStatement = config.Node().Bool(CfgWriteStatement)

	configureFPC()

	// subscribe to FCOB events
	ConsensusMechanism().Events.Vote.Attach(events.NewClosure(func(id string, initOpn opinion.Opinion) {
		if err := Voter().Vote(id, vote.ConflictType, initOpn); err != nil {
			consensusPluginLog.Warnf("FPC vote: %s", err)
		}
	}))
	ConsensusMechanism().Events.Error.Attach(events.NewClosure(func(err error) {
		consensusPluginLog.Errorf("FCOB error: %s", err)
	}))

	// subscribe to message-layer
	Tangle().ConsensusManager.Events.MessageOpinionFormed.Attach(events.NewClosure(readStatement))
}

func runConsensusPlugin(*node.Plugin) {
	runFPC()
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

func configureFPC() {
	if FPCParameters.Listen {
		lPeer := local.GetInstance()
		_, portStr, err := net.SplitHostPort(FPCParameters.BindAddress)
		if err != nil {
			consensusPluginLog.Fatalf("FPC bind address '%s' is invalid: %s", FPCParameters.BindAddress, err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			consensusPluginLog.Fatalf("FPC bind address '%s' is invalid: %s", FPCParameters.BindAddress, err)
		}

		if err := lPeer.UpdateService(service.FPCKey, "tcp", port); err != nil {
			consensusPluginLog.Fatalf("could not update services: %v", err)
		}
	}

	Voter().Events().RoundExecuted.Attach(events.NewClosure(func(roundStats *vote.RoundStats) {
		if writeStatement {
			makeStatement(roundStats)
		}
		peersQueried := len(roundStats.QueriedOpinions)
		voteContextsCount := len(roundStats.ActiveVoteContexts)
		consensusPluginLog.Debugf("executed round with rand %0.4f for %d vote contexts on %d peers, took %v", roundStats.RandUsed, voteContextsCount, peersQueried, roundStats.Duration)
	}))

	Voter().Events().Finalized.Attach(events.NewClosure(ConsensusMechanism().ProcessVote))
	Voter().Events().Finalized.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		if ev.Ctx.Type == vote.ConflictType {
			consensusPluginLog.Infof("FPC finalized for transaction with id '%s' - final opinion: '%s'", ev.ID, ev.Opinion)
		}
	}))

	Voter().Events().Failed.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		if ev.Ctx.Type == vote.ConflictType {
			consensusPluginLog.Warnf("FPC failed for transaction with id '%s' - last opinion: '%s'", ev.ID, ev.Opinion)
		}
	}))

}

func runFPC() {
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
				consensusPluginLog.Infof("%s started, bind-address=%s", ServerWorkerName, bindAddr)
				if err := voterServer.Run(); err != nil {
					consensusPluginLog.Errorf("Error serving: %s", err)
				}
				close(stopped)
			}()

			// stop if we are shutting down or the server could not be started
			select {
			case <-shutdownSignal:
			case <-stopped:
			}

			consensusPluginLog.Infof("Stopping %s ...", ServerWorkerName)
			voterServer.Shutdown()
			consensusPluginLog.Infof("Stopping %s ... done", ServerWorkerName)
		}, shutdown.PriorityFPC); err != nil {
			consensusPluginLog.Panicf("Failed to start as daemon: %s", err)
		}
	}

	if err := daemon.BackgroundWorker("FPCRoundsInitiator", func(shutdownSignal <-chan struct{}) {
		consensusPluginLog.Infof("Started FPC round initiator")
		defer consensusPluginLog.Infof("Stopped FPC round initiator")
		unixTsPRNG := prng.NewUnixTimestampPRNG(FPCParameters.RoundInterval)
		unixTsPRNG.Start()
		defer unixTsPRNG.Stop()
	exit:
		for {
			select {
			case r := <-unixTsPRNG.C():
				if err := voter.Round(r); err != nil {
					consensusPluginLog.Warnf("unable to execute FPC round: %s", err)
				}
			case <-shutdownSignal:
				break exit
			}
		}
	}, shutdown.PriorityFPC); err != nil {
		consensusPluginLog.Panicf("Failed to start as daemon: %s", err)
	}

	if err := daemon.BackgroundWorker("StatementCleaner", func(shutdownSignal <-chan struct{}) {
		consensusPluginLog.Infof("Started Statement Cleaner")
		defer consensusPluginLog.Infof("Stopped Statement Cleaner")
		ticker := time.NewTicker(time.Duration(cleanInterval) * time.Minute)
		defer ticker.Stop()
	exit:
		for {
			select {
			case <-ticker.C:
				Registry().Clean(time.Duration(deleteAfter) * time.Minute)
			case <-shutdownSignal:
				break exit
			}
		}
	}, shutdown.PriorityFPC); err != nil {
		consensusPluginLog.Panicf("Failed to start as daemon: %s", err)
	}
}
