package consensus

import (
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
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
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
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

	// CfgFPCQuerySampleSize defines how many nodes will be queried each round.
	CfgFPCQuerySampleSize = "fpc.querySampleSize"

	// CfgFPCRoundInterval defines how long a round lasts (in seconds)
	CfgFPCRoundInterval = "fpc.roundInterval"

	// CfgFPCListen defines if the FPC service should listen.
	CfgFPCListen = "fpc.listen"

	// CfgFPCBindAddress defines on which address the FPC service should listen.
	CfgFPCBindAddress = "fpc.bindAddress"

	// CfgWaitForStatement is the time in seconds for which the node wait for receiveing the new statement.
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
	flag.Bool(CfgFPCListen, true, "if the FPC service should listen")
	flag.Bool(CfgWriteStatement, false, "if the node should make statements")
	flag.Int(CfgFPCQuerySampleSize, 21, "Size of the voting quorum (k)")
	flag.Int64(CfgFPCRoundInterval, 10, "FPC round interval [s]")
	flag.String(CfgFPCBindAddress, "0.0.0.0:10895", "the bind address on which the FPC vote server binds to")
	flag.Int(CfgWaitForStatement, 5, "the time in seconds for which the node wait for receiveing the new statement")
	flag.Float64(CfgManaThreshold, 1., "Mana threshold to accept/write a statement")
	flag.Int(CfgCleanInterval, 5, "the time in minutes after which the node cleans the statement registry")
	flag.Int(CfgDeleteAfter, 5, "the time in minutes after which older statements are deleted from the registry")
}

var (
	// plugin is the plugin instance of the statement plugin.
	plugin               *node.Plugin
	once                 sync.Once
	voter                *fpc.FPC
	voterOnce            sync.Once
	voterServer          *votenet.VoterServer
	roundIntervalSeconds int64
	log                  *logger.Logger
	registry             *statement.Registry
	registryOnce         sync.Once
	waitForStatement     int
	listen               bool
	cleanInterval        int
	deleteAfter          int
	writeStatement       bool
)

// Plugin returns the consensus plugin.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(ConsensusPluginName, node.Enabled, configure, run)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(ConsensusPluginName)

	configureRemoteLogger()

	roundIntervalSeconds = config.Node().Int64(CfgFPCRoundInterval)
	waitForStatement = config.Node().Int(CfgWaitForStatement)
	listen = config.Node().Bool(CfgFPCListen)
	cleanInterval = config.Node().Int(CfgCleanInterval)
	deleteAfter = config.Node().Int(CfgDeleteAfter)
	writeStatement = config.Node().Bool(CfgWriteStatement)

	configureFPC()

	// subscribe to FCOB events
	valuetransfers.FCOB().Events.Vote.Attach(events.NewClosure(func(id string, initOpn opinion.Opinion) {
		if err := Voter().Vote(id, vote.ConflictType, initOpn); err != nil {
			log.Warnf("FPC vote: %s", err)
		}
	}))
	valuetransfers.FCOB().Events.Error.Attach(events.NewClosure(func(err error) {
		log.Errorf("FCOB error: %s", err)
	}))

	// subscribe to message-layer
	messagelayer.Tangle().Scheduler.Events.MessageScheduled.Attach(events.NewClosure(readStatement))
}

func run(_ *node.Plugin) {
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
	if listen {
		lPeer := local.GetInstance()
		bindAddr := config.Node().String(CfgFPCBindAddress)
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
	}

	Voter().Events().RoundExecuted.Attach(events.NewClosure(func(roundStats *vote.RoundStats) {
		if writeStatement {
			makeStatement(roundStats)
		}
		peersQueried := len(roundStats.QueriedOpinions)
		voteContextsCount := len(roundStats.ActiveVoteContexts)
		log.Debugf("executed round with rand %0.4f for %d vote contexts on %d peers, took %v", roundStats.RandUsed, voteContextsCount, peersQueried, roundStats.Duration)
	}))

	Voter().Events().Finalized.Attach(events.NewClosure(valuetransfers.FCOB().ProcessVoteResult))
	Voter().Events().Finalized.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		if ev.Ctx.Type == vote.ConflictType {
			log.Infof("FPC finalized for transaction with id '%s' - final opinion: '%s'", ev.ID, ev.Opinion)
		}
	}))

	Voter().Events().Failed.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		if ev.Ctx.Type == vote.ConflictType {
			log.Warnf("FPC failed for transaction with id '%s' - last opinion: '%s'", ev.ID, ev.Opinion)
		}
	}))

}

func runFPC() {
	const ServerWorkerName = "FPCVoterServer"

	if listen {
		if err := daemon.BackgroundWorker(ServerWorkerName, func(shutdownSignal <-chan struct{}) {
			stopped := make(chan struct{})
			bindAddr := config.Node().String(CfgFPCBindAddress)
			voterServer = votenet.New(Voter(), OpinionRetriever, bindAddr,
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

	if err := daemon.BackgroundWorker("StatementCleaner", func(shutdownSignal <-chan struct{}) {
		log.Infof("Started Statement Cleaner")
		defer log.Infof("Stopped Statement Cleaner")
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
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
