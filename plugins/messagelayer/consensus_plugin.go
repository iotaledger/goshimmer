package messagelayer

import (
	"context"
	"fmt"
	"github.com/iotaledger/goshimmer/packages/mana"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/consensus/fcob"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/prng"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/fpc"
	votenet "github.com/iotaledger/goshimmer/packages/vote/net"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/goshimmer/packages/vote/statement"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/clock"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/node"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

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
		voter = fpc.New(OpinionGiverFunc, OwnManaRetriever)
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OpinionGivers ////////////////////////////////////////////////////////////////////////////////////////////////

// OpinionGiver is a wrapper for both statements and peers.
type OpinionGiver struct {
	id   identity.ID
	view *statement.View
	pog  *PeerOpinionGiver
	mana float64
}

// OpinionGivers is a map of OpinionGiver.
type OpinionGivers map[identity.ID]OpinionGiver

// Query retrieves the opinions about the given conflicts and timestamps.
func (o *OpinionGiver) Query(ctx context.Context, conflictIDs []string, timestampIDs []string) (opinions opinion.Opinions, err error) {
	for i := 0; i < StatementParameters.WaitForStatement; i++ {
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

// Mana returns consensus mana value for an opinion giver
func (o *OpinionGiver) Mana() float64 {
	return o.mana
}

// OpinionGiverFunc returns a slice of opinion givers.
func OpinionGiverFunc() (givers []opinion.OpinionGiver, err error) {
	opinionGiversMap := make(map[identity.ID]*OpinionGiver)
	opinionGivers := make([]opinion.OpinionGiver, 0)

	consensusManaNodes, _, err := GetManaMap(mana.ConsensusMana)

	for _, v := range Registry().NodesView() {
		manaValue := 0.0
		if v, ok := consensusManaNodes[v.ID()]; ok {
			manaValue = v
		}
		opinionGiversMap[v.ID()] = &OpinionGiver{
			id:   v.ID(),
			view: v,
			mana: manaValue,
		}
	}

	for _, p := range autopeering.Discovery().GetVerifiedPeers() {
		fpcService := p.Services().Get(service.FPCKey)
		if fpcService == nil {
			continue
		}
		if _, ok := opinionGiversMap[p.ID()]; !ok {
			manaValue := 0.0
			if v, ok := consensusManaNodes[p.ID()]; ok {
				manaValue = v
			}
			opinionGiversMap[p.ID()] = &OpinionGiver{
				id:   p.ID(),
				view: nil,
				mana: manaValue,
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

// region PeerOpinionGiver /////////////////////////////////////////////////////////////////////////////////////////////

// PeerOpinionGiver implements the OpinionGiver interface based on a peer.
type PeerOpinionGiver struct {
	p *peer.Peer
}

// Query queries another node for its opinion.
func (pog *PeerOpinionGiver) Query(ctx context.Context, conflictIDs []string, timestampIDs []string) (opinion.Opinions, error) {
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
	opinions := make(opinion.Opinions, len(reply.Opinion))
	for i, intOpn := range reply.Opinion {
		opinions[i] = opinion.ConvertInt32Opinion(intOpn)
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

// region OwnWeightsRetriever/////////////////////////////////////////////////////////////////////////////////////

// OwnManaRetriever returns the current consensus mana of a vector
func OwnManaRetriever() (float64, error) {
	var ownMana float64
	consensusManaNodes, _, err := GetManaMap(mana.ConsensusMana)
	if v, ok := consensusManaNodes[local.GetInstance().ID()]; ok {
		ownMana = v
	}
	return ownMana, err
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////////

// region OpinionRetriever /////////////////////////////////////////////////////////////////////////////////////////////

// OpinionRetriever returns the current opinion of the given id.
func OpinionRetriever(id string, objectType vote.ObjectType) opinion.Opinion {
	switch objectType {
	case vote.TimestampType:
		// TODO: implement
		return opinion.Like
	default: // conflict type
		transactionID, err := ledgerstate.TransactionIDFromBase58(id)
		if err != nil {
			plugin.LogErrorf("received invalid vote request for branch '%s'", id)

			return opinion.Unknown
		}

		opinionEssence := ConsensusMechanism().TransactionOpinionEssence(transactionID)

		if opinionEssence.LevelOfKnowledge() == fcob.Pending {
			return opinion.Unknown
		}

		if !opinionEssence.Liked() {
			return opinion.Dislike
		}

		return opinion.Like
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RemoteLogger /////////////////////////////////////////////////////////////////////////////////////////////////

const (
	remoteLogType = "statement"
)

var (
	remoteLogger *remotelog.RemoteLoggerConn
	myID         string
	clockEnabled bool
)

func configureRemoteLogger() {
	remoteLogger = remotelog.RemoteLogger()

	if local.GetInstance() != nil {
		myID = local.GetInstance().ID().String()
	}

	clockEnabled = !node.IsSkipped(clock.Plugin())
}

func sendToRemoteLog(msgID, issuerID string, issuedTime, arrivalTime, solidTime int64) {
	m := statementLog{
		NodeID:       myID,
		MsgID:        msgID,
		IssuerID:     issuerID,
		IssuedTime:   issuedTime,
		ArrivalTime:  arrivalTime,
		SolidTime:    solidTime,
		DeltaArrival: arrivalTime - issuedTime,
		DeltaSolid:   solidTime - issuedTime,
		Clock:        clockEnabled,
		Sync:         Tangle().Synced(),
		Type:         remoteLogType,
	}
	_ = remoteLogger.Send(m)
}

type statementLog struct {
	NodeID       string `json:"nodeID"`
	MsgID        string `json:"msgID"`
	IssuerID     string `json:"issuerID"`
	IssuedTime   int64  `json:"issuedTime"`
	ArrivalTime  int64  `json:"arrivalTime"`
	SolidTime    int64  `json:"solidTime"`
	DeltaArrival int64  `json:"deltaArrival"`
	DeltaSolid   int64  `json:"deltaSolid"`
	Clock        bool   `json:"clock"`
	Sync         bool   `json:"sync"`
	Type         string `json:"type"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Statement ////////////////////////////////////////////////////////////////////////////////////////////////////

func makeStatement(roundStats *vote.RoundStats) {
	// TODO: add check for Mana threshold

	timestamps := statement.Timestamps{}
	conflicts := statement.Conflicts{}

	for id, v := range roundStats.ActiveVoteContexts {
		switch v.Type {
		case vote.TimestampType:
			messageID, err := tangle.NewMessageID(id)
			if err != nil {
				// TODO
				break
			}
			timestamps = append(timestamps, statement.Timestamp{
				ID: messageID,
				Opinion: statement.Opinion{
					Value: v.LastOpinion(),
					Round: uint8(v.Rounds)}},
			)
		case vote.ConflictType:
			messageID, err := ledgerstate.TransactionIDFromBase58(id)
			if err != nil {
				// TODO
				break
			}
			conflicts = append(conflicts, statement.Conflict{
				ID: messageID,
				Opinion: statement.Opinion{
					Value: v.LastOpinion(),
					Round: uint8(v.Rounds)}},
			)
		default:
		}
	}

	broadcastStatement(conflicts, timestamps)
}

// broadcastStatement broadcasts a statement via communication layer.
func broadcastStatement(conflicts statement.Conflicts, timestamps statement.Timestamps) {
	msg, err := Tangle().IssuePayload(statement.New(conflicts, timestamps))

	if err != nil {
		plugin.LogWarnf("error issuing statement: %s", err)
		return
	}

	plugin.LogDebugf("issued statement %s", msg.ID())
}

func readStatement(messageID tangle.MessageID) {
	Tangle().Storage.Message(messageID).Consume(func(msg *tangle.Message) {
		messagePayload := msg.Payload()
		if messagePayload.Type() != statement.StatementType {
			return
		}
		statementPayload, ok := messagePayload.(*statement.Statement)
		if !ok {
			plugin.LogDebug("could not cast payload to statement object")
			return
		}

		// TODO: check if the Mana threshold of the issuer is ok

		issuerID := identity.NewID(msg.IssuerPublicKey())
		// Skip ourselves
		if issuerID == local.GetInstance().ID() {
			return
		}

		issuerRegistry := Registry().NodeView(issuerID)

		issuerRegistry.AddConflicts(statementPayload.Conflicts)

		issuerRegistry.AddTimestamps(statementPayload.Timestamps)

		Tangle().Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
			sendToRemoteLog(
				msg.ID().String(),
				issuerID.String(),
				msg.IssuingTime().UnixNano(),
				messageMetadata.ReceivedTime().UnixNano(),
				messageMetadata.SolidificationTime().UnixNano(),
			)
		})
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
