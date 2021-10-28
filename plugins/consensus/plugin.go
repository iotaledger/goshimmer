package consensus

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	clockPkg "github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/consensus/fcob"
	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/fpc"
	votenet "github.com/iotaledger/goshimmer/packages/vote/net"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/goshimmer/packages/vote/statement"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

var (
	// Plugin is the plugin instance of the consensus plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Tangle             *tangle.Tangle
	Local              *peer.Local
	Discover           *discover.Protocol       `optional:"true"`
	Voter              vote.DRNGRoundBasedVoter `optional:"true"`
	VoterServer        *votenet.VoterServer
	ConsensusMechanism tangle.ConsensusMechanism
	Registry           *statement.Registry
	DRNGTicker         *drng.Ticker
}

func init() {
	Plugin = node.NewPlugin("Consensus", deps, node.Enabled, configureConsensusPlugin, runConsensusPlugin)

	Plugin.Events.Init.Attach(events.NewClosure(func(_ *node.Plugin, container *dig.Container) {
		if err := container.Provide(func() vote.DRNGRoundBasedVoter {
			return fpc.New(OpinionGiverFunc, OwnManaRetriever)
		}); err != nil {
			Plugin.Panic(err)
		}

		if err := container.Provide(func(voter vote.DRNGRoundBasedVoter) *votenet.VoterServer {
			return votenet.New(voter, OpinionRetriever, FPCParameters.BindAddress,
				metrics.Events().FPCInboundBytes,
				metrics.Events().FPCOutboundBytes,
				metrics.Events().QueryReceived,
			)
		}); err != nil {
			Plugin.Panic(err)
		}

		if err := container.Provide(statement.NewRegistry); err != nil {
			Plugin.Panic(err)
		}

		if err := container.Provide(func(state *drng.State) *drng.Ticker {
			return drng.NewTicker(func() *drng.State { return state }, FPCParameters.RoundInterval, FPCParameters.DefaultRandomness, FPCParameters.AwaitOffset)
		}); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configureConsensusPlugin(plugin *node.Plugin) {
	configureFPC(plugin)

	// subscribe to FCOB events
	consensusMechanism := deps.ConsensusMechanism.(*fcob.ConsensusMechanism)
	consensusMechanism.Events.Vote.Attach(events.NewClosure(func(id string, initOpn opinion.Opinion) {
		if deps.Voter != nil {
			if err := deps.Voter.Vote(id, vote.ConflictType, initOpn); err != nil {
				plugin.LogWarnf("FPC vote: %s", err)
			}
		}
	}))
	consensusMechanism.Events.Error.Attach(events.NewClosure(func(err error) {
		plugin.LogErrorf("FCOB error: %s", err)
	}))

	// subscribe to message-layer
	deps.Tangle.ConsensusManager.Events.MessageOpinionFormed.Attach(events.NewClosure(readStatement))
}

func runConsensusPlugin(plugin *node.Plugin) {
	runFPC(plugin)
}

func configureFPC(plugin *node.Plugin) {
	if FPCParameters.Listen {
		lPeer := deps.Local
		_, portStr, err := net.SplitHostPort(FPCParameters.BindAddress)
		if err != nil {
			plugin.LogFatalf("FPC bind address '%s' is invalid: %s", FPCParameters.BindAddress, err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			plugin.LogFatalf("FPC bind address '%s' is invalid: %s", FPCParameters.BindAddress, err)
		}

		if err = lPeer.UpdateService(service.FPCKey, "tcp", port); err != nil {
			plugin.LogFatalf("could not update services: %v", err)
		}
	}

	if deps.Voter != nil {
		deps.Voter.Events().RoundExecuted.Attach(events.NewClosure(func(roundStats *vote.RoundStats) {
			if StatementParameters.WriteStatement && checkEnoughMana(deps.Local.ID(), StatementParameters.WriteManaThreshold) {
				makeStatement(roundStats, broadcastStatement)
			}
			peersQueried := len(roundStats.QueriedOpinions)
			voteContextsCount := len(roundStats.ActiveVoteContexts)
			plugin.LogDebugf("executed round with rand %0.4f for %d vote contexts on %d peers, took %v", roundStats.RandUsed, voteContextsCount, peersQueried, roundStats.Duration)
		}))

		consensusMechanism := deps.ConsensusMechanism.(*fcob.ConsensusMechanism)
		deps.Voter.Events().Finalized.Attach(events.NewClosure(consensusMechanism.ProcessVote))
		deps.Voter.Events().Finalized.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
			if ev.Ctx.Type == vote.ConflictType {
				plugin.LogInfof("FPC finalized for transaction with id '%s' - final opinion: '%s'", ev.ID, ev.Opinion)
			}
		}))

		deps.Voter.Events().Failed.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
			if ev.Ctx.Type == vote.ConflictType {
				plugin.LogWarnf("FPC failed for transaction with id '%s' - last opinion: '%s'", ev.ID, ev.Opinion)
			}
		}))
	}
}

func runFPC(plugin *node.Plugin) {
	const ServerWorkerName = "FPCVoterServer"

	if FPCParameters.Listen {
		if err := daemon.BackgroundWorker(ServerWorkerName, func(ctx context.Context) {
			stopped := make(chan struct{})
			bindAddr := FPCParameters.BindAddress

			go func() {
				plugin.LogInfof("%s started, bind-address=%s", ServerWorkerName, bindAddr)
				if err := deps.VoterServer.Run(); err != nil {
					plugin.LogErrorf("Error serving: %s", err)
				}
				close(stopped)
			}()

			// stop if we are shutting down or the server could not be started
			select {
			case <-ctx.Done():
			case <-stopped:
			}

			plugin.LogInfof("Stopping %s ...", ServerWorkerName)
			deps.VoterServer.Shutdown()
			plugin.LogInfof("Stopping %s ... done", ServerWorkerName)
		}, shutdown.PriorityFPC); err != nil {
			plugin.Panicf("Failed to start as daemon: %s", err)
		}
	}

	if err := daemon.BackgroundWorker("FPCRoundsInitiator", func(ctx context.Context) {
		plugin.LogInfof("Started FPC round initiator")
		defer plugin.LogInfof("Stopped FPC round initiator")

		deps.DRNGTicker.Start()
	exit:
		for {
			select {
			case r := <-deps.DRNGTicker.C():
				if err := deps.Voter.Round(r, deps.DRNGTicker.DelayedRoundStart()); err != nil {
					plugin.LogWarnf("unable to execute FPC round: %s", err)
				}
			case <-ctx.Done():
				deps.DRNGTicker.Stop()
				break exit
			}
		}
	}, shutdown.PriorityFPC); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}

	if err := daemon.BackgroundWorker("StatementCleaner", func(ctx context.Context) {
		plugin.LogInfof("Started Statement Cleaner")
		defer plugin.LogInfof("Stopped Statement Cleaner")
		ticker := time.NewTicker(StatementParameters.CleanInterval)
		defer ticker.Stop()
	exit:
		for {
			select {
			case <-ticker.C:
				deps.Registry.Clean(StatementParameters.DeleteAfter)
			case <-ctx.Done():
				break exit
			}
		}
	}, shutdown.PriorityFPC); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

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
func (o *OpinionGiver) Query(ctx context.Context, conflictIDs, timestampIDs []string, delayedRoundStart ...time.Duration) (opinions opinion.Opinions, err error) {
	waitForStatements := StatementParameters.WaitForStatement
	// delayedRoundStart gives the time that has elapsed since the start of the current round.
	if len(delayedRoundStart) != 0 {
		if delayedRoundStart[0] < waitForStatements && delayedRoundStart[0] > 0 {
			waitForStatements -= delayedRoundStart[0]
		}
	}

	// if o.view == nil, then we can immediately perform P2P query instead of waiting for statement
	// because it won't be provided.
	if o.view != nil {
		// wait for statement(s) to arrive
		time.Sleep(waitForStatements)

		// check if node has been active in the last two rounds
		// note, we cannot simply set one RoundInterval since the last message could e.g. have arrived 1.5 intervals ago
		if o.view.LastStatementReceivedTimestamp.Add(2 * FPCParameters.RoundInterval).After(clockPkg.SyncedTime()) {
			opinions, err = o.view.Query(ctx, conflictIDs, timestampIDs)
			if err == nil {
				return opinions, nil
			}
		}
	}

	// query node directly
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

	consensusManaNodes, _, err := messagelayer.GetManaMap(mana.ConsensusMana)
	if err != nil {
		Plugin.LogErrorf("Error retrieving consensus mana: %s", err)
	}
	for _, v := range deps.Registry.NodesView() {
		// double check to exclude self
		if v.ID() == deps.Local.ID() {
			continue
		}

		manaValue := 0.0

		if manaAmount, ok := consensusManaNodes[v.ID()]; ok {
			manaValue = manaAmount
		}
		opinionGiversMap[v.ID()] = &OpinionGiver{
			id:   v.ID(),
			view: v,
			mana: manaValue,
		}
	}
	if deps.Discover == nil {
		return
	}
	for _, p := range deps.Discover.GetVerifiedPeers() {
		fpcService := p.Services().Get(service.FPCKey)
		if fpcService == nil {
			continue
		}
		if _, ok := opinionGiversMap[p.ID()]; !ok {
			// double check to exclude self
			if p.ID() == deps.Local.ID() {
				continue
			}
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

// PeerOpinionGiver implements the OpinionGiver interface based on a peer.
type PeerOpinionGiver struct {
	p *peer.Peer
}

// Query queries another node for its opinion.
func (pog *PeerOpinionGiver) Query(ctx context.Context, conflictIDs, timestampIDs []string, _ ...time.Duration) (opinion.Opinions, error) {
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
	defer func() {
		cerr := conn.Close()
		if err == nil {
			err = errors.Errorf("failed to close connection: %w", cerr)
		}
	}()

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

	return opinions, err
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

// OwnManaRetriever returns the current consensus mana of a vector
func OwnManaRetriever() (float64, error) {
	var ownMana float64
	consensusManaNodes, _, err := messagelayer.GetManaMap(mana.ConsensusMana)
	if v, ok := consensusManaNodes[deps.Local.ID()]; ok {
		ownMana = v
	}
	return ownMana, err
}

// OpinionRetriever returns the current opinion of the given id.
func OpinionRetriever(id string, objectType vote.ObjectType) opinion.Opinion {
	switch objectType {
	case vote.TimestampType:
		// TODO: implement
		return opinion.Like
	default: // conflict type
		transactionID, err := ledgerstate.TransactionIDFromBase58(id)
		if err != nil {
			Plugin.LogErrorf("received invalid vote request for branch '%s'", id)

			return opinion.Unknown
		}

		consensusMechanism := deps.ConsensusMechanism.(*fcob.ConsensusMechanism)
		opinionEssence := consensusMechanism.TransactionOpinionEssence(transactionID)

		if opinionEssence.LevelOfKnowledge() == fcob.Pending {
			return opinion.Unknown
		}

		if !opinionEssence.Liked() {
			return opinion.Dislike
		}

		return opinion.Like
	}
}

const (
	maxPayloadRatio = 0.9
)

// checkEnoughMana function check whether a node with id is among the top holders of p percent of consensus mana mana
func checkEnoughMana(id identity.ID, threshold float64) bool {
	highestManaNodes, _, err := messagelayer.GetHighestManaNodesFraction(mana.ConsensusMana, threshold)
	enoughMana := true
	if err == nil && threshold < 1.0 && len(highestManaNodes) > 0 {
		enoughMana = false
		for _, v := range highestManaNodes {
			if v.ID == id {
				enoughMana = true
				break
			}
		}
	}
	return enoughMana
}

func makeStatement(roundStats *vote.RoundStats, broadcastFunc func(conflicts statement.Conflicts, timestamps statement.Timestamps)) {
	timestamps := statement.Timestamps{}
	conflicts := statement.Conflicts{}

	for id, v := range roundStats.ActiveVoteContexts {
		switch v.Type {
		case vote.TimestampType:
			timeStampStatement, err := makeTimeStampStatement(id, v)
			if err != nil {
				Plugin.LogErrorf("Statement error: %s", errors.Errorf("Failed to create a TimeStamp statement: %w", err))
				break
			}
			timestamps = append(timestamps, timeStampStatement)
		case vote.ConflictType:
			conflictStatement, err := makeConflictStatement(id, v)
			if err != nil {
				Plugin.LogErrorf("Statement error: %s", errors.Errorf("Failed to create a Conflict statement: %w", err))
				break
			}
			conflicts = append(conflicts, conflictStatement)
		}
		conflicts, timestamps = handleStatement(conflicts, timestamps, broadcastFunc)
	}

	if len(conflicts)+len(timestamps) >= 0 {
		broadcastFunc(conflicts, timestamps)
	}
}

// handleStatement limits the size of statements if size exceeds max capacity
func handleStatement(conflicts statement.Conflicts, timestamps statement.Timestamps,
	broadcastFunc func(conflicts statement.Conflicts, timestamps statement.Timestamps)) (statement.Conflicts, statement.Timestamps) {
	if hasStatementExceededMaxSize(conflicts, timestamps) {
		broadcastFunc(conflicts, timestamps)
		timestamps = statement.Timestamps{}
		conflicts = statement.Conflicts{}
	}
	return conflicts, timestamps
}

func hasStatementExceededMaxSize(conflicts statement.Conflicts, timestamps statement.Timestamps) bool {
	maxSize := payload.MaxSize
	return (len(conflicts)*statement.ConflictLength + len(timestamps)*statement.TimestampLength) >= int(maxPayloadRatio*float64(maxSize))
}

func makeConflictStatement(id string, v *vote.Context) (statement.Conflict, error) {
	messageID, err := ledgerstate.TransactionIDFromBase58(id)
	if err != nil {
		err = errors.Errorf("Failed to create a Conflict statement: %w", err)
		return statement.Conflict{}, err
	}
	conflict := statement.Conflict{
		ID: messageID,
		Opinion: statement.Opinion{
			Value: v.LastOpinion(),
			Round: uint8(v.Rounds),
		},
	}
	return conflict, nil
}

func makeTimeStampStatement(id string, v *vote.Context) (statement.Timestamp, error) {
	messageID, err := tangle.NewMessageID(id)
	if err != nil {
		err = errors.Errorf("Failed to create a TimeStamp statement: %w", err)
		return statement.Timestamp{}, err
	}
	timestamp := statement.Timestamp{
		ID: messageID,
		Opinion: statement.Opinion{
			Value: v.LastOpinion(),
			Round: uint8(v.Rounds),
		},
	}
	return timestamp, nil
}

// broadcastStatement broadcasts a statement via communication layer.
func broadcastStatement(conflicts statement.Conflicts, timestamps statement.Timestamps) {
	msg, err := deps.Tangle.IssuePayload(statement.New(conflicts, timestamps))
	if err != nil {
		Plugin.LogWarnf("error issuing statement: %s", err)
		return
	}

	Plugin.LogDebugf("issued statement %s", msg.ID())
}

func readStatement(messageID tangle.MessageID) {
	deps.Tangle.Storage.Message(messageID).Consume(func(msg *tangle.Message) {
		messagePayload := msg.Payload()
		if messagePayload.Type() != statement.StatementType {
			return
		}
		statementPayload, ok := messagePayload.(*statement.Statement)
		if !ok {
			Plugin.LogDebug("could not cast payload to statement object")
			return
		}

		issuerID := identity.NewID(msg.IssuerPublicKey())

		// check if the Mana threshold of the issuer is ok
		if !checkEnoughMana(issuerID, StatementParameters.ReadManaThreshold) {
			return
		}
		// Skip ourselves
		if issuerID == deps.Local.ID() {
			return
		}

		issuerRegistry := deps.Registry.NodeView(issuerID)
		issuerRegistry.AddConflicts(statementPayload.Conflicts)
		issuerRegistry.AddTimestamps(statementPayload.Timestamps)
		issuerRegistry.UpdateLastStatementReceivedTime(clockPkg.SyncedTime())

		deps.Tangle.ConsensusManager.Events.StatementProcessed.Trigger(msg)
	})
}
