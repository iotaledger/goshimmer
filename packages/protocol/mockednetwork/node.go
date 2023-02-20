package mockednetwork

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/filter"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/enginemanager"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/workerpool"
)

type Node struct {
	Testing             *testing.T
	Name                string
	KeyPair             ed25519.KeyPair
	Endpoint            *network.MockedEndpoint
	Workers             *workerpool.Group
	Protocol            *protocol.Protocol
	EngineTestFramework *engine.TestFramework
}

func NewNode(t *testing.T, keyPair ed25519.KeyPair, network *network.MockedNetwork, partition string, snapshotPath string, engineOpts ...options.Option[engine.Engine]) *Node {
	id := identity.New(keyPair.PublicKey)

	node := &Node{
		Testing:  t,
		Workers:  workerpool.NewGroup(id.ID().String()),
		Name:     id.ID().String(),
		KeyPair:  keyPair,
		Endpoint: network.Join(id.ID(), partition),
	}

	tempDir := utils.NewDirectory(t.TempDir())

	node.Protocol = protocol.New(node.Workers.CreateGroup("Protocol"),
		node.Endpoint,
		protocol.WithBaseDirectory(tempDir.Path()),
		protocol.WithSnapshotPath(snapshotPath),
		protocol.WithEngineOptions(engineOpts...),
	)
	node.Protocol.Run()

	t.Cleanup(func() {
		fmt.Println(node.Name, "> shutdown")
		node.Protocol.Shutdown()
		fmt.Println(node.Name, "> shutdown done")
	})

	mainEngine := node.Protocol.MainEngineInstance()
	node.EngineTestFramework = engine.NewTestFramework(t, node.Workers.CreateGroup(fmt.Sprintf("EngineTestFramework-%s", mainEngine.Name()[:8])), mainEngine.Engine)

	return node
}

func (n *Node) HookLogging(includeMainEngine bool) {
	events := n.Protocol.Events

	if includeMainEngine {
		n.attachEngineLogs(n.Protocol.MainEngineInstance())
	}

	event.Hook(events.CandidateEngineActivated, func(candidateEngine *enginemanager.EngineInstance) {
		n.EngineTestFramework = engine.NewTestFramework(n.Testing, n.Workers.CreateGroup(fmt.Sprintf("EngineTestFramework-%s", candidateEngine.Name()[:8])), candidateEngine.Engine)

		fmt.Printf("%s > CandidateEngineActivated: latest commitment %s %s\n", n.Name, candidateEngine.Storage.Settings.LatestCommitment().ID(), candidateEngine.Storage.Settings.LatestCommitment())
		fmt.Printf("==================\nACTIVATE %s\n==================\n", n.Name)
		n.attachEngineLogs(candidateEngine)
	})

	event.Hook(events.MainEngineSwitched, func(engine *enginemanager.EngineInstance) {
		fmt.Printf("%s > MainEngineSwitched: latest commitment %s %s\n", n.Name, engine.Storage.Settings.LatestCommitment().ID(), engine.Storage.Settings.LatestCommitment())
		fmt.Printf("================\nSWITCH %s\n================\n", n.Name)
	})

	event.Hook(events.CongestionControl.Scheduler.BlockScheduled, func(block *scheduler.Block) {
		fmt.Printf("%s > CongestionControl.Scheduler.BlockScheduled: %s\n", n.Name, block.ID())
	})

	event.Hook(events.CongestionControl.Scheduler.BlockDropped, func(block *scheduler.Block) {
		fmt.Printf("%s > CongestionControl.Scheduler.BlockDropped: %s\n", n.Name, block.ID())
	})

	event.Hook(events.CongestionControl.Scheduler.BlockSubmitted, func(block *scheduler.Block) {
		fmt.Printf("%s > CongestionControl.Scheduler.BlockSubmitted: %s\n", n.Name, block.ID())
	})

	event.Hook(events.CongestionControl.Scheduler.BlockSkipped, func(block *scheduler.Block) {
		fmt.Printf("%s > CongestionControl.Scheduler.BlockSkipped: %s\n", n.Name, block.ID())
	})

	event.Hook(events.ChainManager.ForkDetected, func(fork *chainmanager.Fork) {
		fmt.Printf("%s > ChainManager.ForkDetected: %s with forking point %s received from %s\n", n.Name, fork.Commitment.ID(), fork.ForkingPoint.ID(), fork.Source)
		fmt.Printf("----------------------\nForkDetected %s\n----------------------\n", n.Name)
	})

	event.Hook(events.Error, func(err error) {
		fmt.Printf("%s > Error: %s\n", n.Name, err.Error())
	})

	event.Hook(events.Network.BlockReceived, func(event *network.BlockReceivedEvent) {
		fmt.Printf("%s > Network.BlockReceived: from %s %s - %d\n", n.Name, event.Source, event.Block.ID(), event.Block.ID().Index())
	})

	event.Hook(events.Network.BlockRequestReceived, func(event *network.BlockRequestReceivedEvent) {
		fmt.Printf("%s > Network.BlockRequestReceived: from %s %s\n", n.Name, event.Source, event.BlockID)
	})

	event.Hook(events.Network.AttestationsReceived, func(event *network.AttestationsReceivedEvent) {
		fmt.Printf("%s > Network.AttestationsReceived: from %s for %s\n", n.Name, event.Source, event.Commitment.ID())
	})

	event.Hook(events.Network.AttestationsRequestReceived, func(event *network.AttestationsRequestReceivedEvent) {
		fmt.Printf("%s > Network.AttestationsRequestReceived: from %s %s -> %d\n", n.Name, event.Source, event.Commitment.ID(), event.EndIndex)
	})

	event.Hook(events.Network.EpochCommitmentReceived, func(event *network.EpochCommitmentReceivedEvent) {
		fmt.Printf("%s > Network.EpochCommitmentReceived: from %s %s\n", n.Name, event.Source, event.Commitment.ID())
	})

	event.Hook(events.Network.EpochCommitmentRequestReceived, func(event *network.EpochCommitmentRequestReceivedEvent) {
		fmt.Printf("%s > Network.EpochCommitmentRequestReceived: from %s %s\n", n.Name, event.Source, event.CommitmentID)
	})

	event.Hook(events.Network.Error, func(event *network.ErrorEvent) {
		fmt.Printf("%s > Network.Error: from %s %s\n", n.Name, event.Source, event.Error.Error())
	})
}

func (n *Node) attachEngineLogs(instance *enginemanager.EngineInstance) {
	engineName := fmt.Sprintf("%s - %s", lo.Cond(n.Protocol.Engine() != instance.Engine, "Candidate", "Main"), instance.Name()[:8])
	events := instance.Engine.Events

	event.Hook(events.Tangle.BlockDAG.BlockAttached, func(block *blockdag.Block) {
		fmt.Printf("%s > [%s] BlockDAG.BlockAttached: %s\n", n.Name, engineName, block.ID())
	})

	event.Hook(events.Tangle.BlockDAG.BlockSolid, func(block *blockdag.Block) {
		fmt.Printf("%s > [%s] BlockDAG.BlockSolid: %s\n", n.Name, engineName, block.ID())
	})

	event.Hook(events.Tangle.BlockDAG.BlockInvalid, func(event *blockdag.BlockInvalidEvent) {
		fmt.Printf("%s > [%s] BlockDAG.BlockInvalid: %s - %s\n", n.Name, engineName, event.Block.ID(), event.Reason.Error())
	})

	event.Hook(events.Tangle.BlockDAG.BlockMissing, func(block *blockdag.Block) {
		fmt.Printf("%s > [%s] BlockDAG.BlockMissing: %s\n", n.Name, engineName, block.ID())
	})

	event.Hook(events.Tangle.BlockDAG.MissingBlockAttached, func(block *blockdag.Block) {
		fmt.Printf("%s > [%s] BlockDAG.MissingBlockAttached: %s\n", n.Name, engineName, block.ID())
	})

	event.Hook(events.Tangle.BlockDAG.BlockOrphaned, func(block *blockdag.Block) {
		fmt.Printf("%s > [%s] BlockDAG.BlockOrphaned: %s\n", n.Name, engineName, block.ID())
	})

	event.Hook(events.Tangle.BlockDAG.BlockUnorphaned, func(block *blockdag.Block) {
		fmt.Printf("%s > [%s] BlockDAG.BlockUnorphaned: %s\n", n.Name, engineName, block.ID())
	})

	event.Hook(events.Tangle.Booker.BlockBooked, func(evt *booker.BlockBookedEvent) {
		fmt.Printf("%s > [%s] Booker.BlockBooked: %s\n", n.Name, engineName, evt.Block.ID())
	})

	event.Hook(events.Tangle.Booker.VirtualVoting.SequenceTracker.VotersUpdated, func(event *sequencetracker.VoterUpdatedEvent) {
		fmt.Printf("%s > [%s] Tangle.VirtualVoting.SequenceTracker.VotersUpdated: %s %s %d -> %d\n", n.Name, engineName, event.Voter, event.SequenceID, event.PrevMaxSupportedIndex, event.NewMaxSupportedIndex)
	})

	event.Hook(events.Clock.AcceptanceTimeUpdated, func(event *clock.TimeUpdateEvent) {
		fmt.Printf("%s > [%s] Clock.AcceptanceTimeUpdated: %s\n", n.Name, engineName, event.NewTime)
	})

	event.Hook(events.Filter.BlockAllowed, func(block *models.Block) {
		fmt.Printf("%s > [%s] Filter.BlockAllowed: %s\n", n.Name, engineName, block.ID())
	})

	event.Hook(events.Filter.BlockFiltered, func(event *filter.BlockFilteredEvent) {
		fmt.Printf("%s > [%s] Filter.BlockFiltered: %s - %s\n", n.Name, engineName, event.Block.ID(), event.Reason.Error())
		n.Testing.Fatal("no blocks should be filtered")
	})

	event.Hook(events.BlockRequester.Tick, func(blockID models.BlockID) {
		fmt.Printf("%s > [%s] BlockRequester.Tick: %s\n", n.Name, engineName, blockID)
	})

	event.Hook(events.BlockProcessed, func(blockID models.BlockID) {
		fmt.Printf("%s > [%s] Engine.BlockProcessed: %s\n", n.Name, engineName, blockID)
	})

	event.Hook(events.Error, func(err error) {
		fmt.Printf("%s > [%s] Engine.Error: %s\n", n.Name, engineName, err.Error())
	})

	event.Hook(events.NotarizationManager.EpochCommitted, func(details *notarization.EpochCommittedDetails) {
		fmt.Printf("%s > [%s] NotarizationManager.EpochCommitted: %s %s\n", n.Name, engineName, details.Commitment.ID(), details.Commitment)
	})

	event.Hook(events.Consensus.BlockGadget.BlockAccepted, func(block *blockgadget.Block) {
		fmt.Printf("%s > [%s] Consensus.BlockGadget.BlockAccepted: %s %s\n", n.Name, engineName, block.ID(), block.Commitment().ID())
	})

	event.Hook(events.Consensus.BlockGadget.BlockConfirmed, func(block *blockgadget.Block) {
		fmt.Printf("%s > [%s] Consensus.BlockGadget.BlockConfirmed: %s %s\n", n.Name, engineName, block.ID(), block.Commitment().ID())
	})

	event.Hook(events.Consensus.EpochGadget.EpochConfirmed, func(epochIndex epoch.Index) {
		fmt.Printf("%s > [%s] Consensus.EpochGadget.EpochConfirmed: %s\n", n.Name, engineName, epochIndex)
	})
}

func (n *Node) Wait() {
	n.Workers.Wait()
}

func (n *Node) IssueBlockAtEpoch(alias string, epochIndex epoch.Index, parents ...models.BlockID) *models.Block {
	issuingTime := time.Unix(epoch.GenesisTime+int64(epochIndex-1)*epoch.Duration, 0)
	require.True(n.Testing, issuingTime.Before(time.Now()), "issued block is in the current or future epoch")
	n.EngineTestFramework.BlockDAG.CreateAndSignBlock(alias, &n.KeyPair,
		models.WithStrongParents(models.NewBlockIDs(parents...)),
		models.WithIssuingTime(issuingTime),
		models.WithCommitment(n.Protocol.Engine().Storage.Settings.LatestCommitment()),
	)
	n.EngineTestFramework.BlockDAG.IssueBlocks(alias)
	fmt.Println(n.Name, alias, n.Workers)
	block := n.EngineTestFramework.BlockDAG.Block(alias)
	fmt.Printf("%s > IssueBlockAtEpoch: %s with %s\n", n.Name, block.ID(), block.Commitment().ID())
	return block
}

func (n *Node) IssueBlock(alias string, parents ...models.BlockID) *models.Block {
	n.EngineTestFramework.BlockDAG.CreateAndSignBlock(alias, &n.KeyPair,
		models.WithStrongParents(models.NewBlockIDs(parents...)),
		models.WithCommitment(n.Protocol.Engine().Storage.Settings.LatestCommitment()),
	)
	n.EngineTestFramework.BlockDAG.IssueBlocks(alias)
	return n.EngineTestFramework.BlockDAG.Block(alias)
}

func (n *Node) IssueActivity(duration time.Duration) {
	go func() {
		start := time.Now()
		fmt.Println(n.Name, "> Starting activity")
		var counter int
		for {
			tips := n.Protocol.TipManager.Tips(1)
			if !n.issueActivityBlock(fmt.Sprintf("%s.%d", n.Name, counter), tips.Slice()...) {
				fmt.Println(n.Name, "> Stopped activity due to block not being issued")
				return
			}
			counter++
			time.Sleep(1 * time.Second)
			if duration > 0 && time.Since(start) > duration {
				fmt.Println(n.Name, "> Stopped activity after", time.Since(start))
				return
			}
		}
	}()
}

func (n *Node) issueActivityBlock(alias string, parents ...models.BlockID) bool {
	if !n.Protocol.Engine().WasStopped() {
		n.EngineTestFramework.BlockDAG.CreateAndSignBlock(alias, &n.KeyPair,
			models.WithStrongParents(models.NewBlockIDs(parents...)),
			models.WithCommitment(n.Protocol.Engine().Storage.Settings.LatestCommitment()),
		)
		n.EngineTestFramework.BlockDAG.IssueBlocks(alias)

		return true
	}
	return false
}

func (n *Node) ValidateAcceptedBlocks(expectedAcceptedBlocks map[models.BlockID]bool) {
	for blockID, blockExpectedAccepted := range expectedAcceptedBlocks {
		actualBlockAccepted := n.Protocol.Engine().Consensus.BlockGadget.IsBlockAccepted(blockID)
		require.Equal(n.Testing, blockExpectedAccepted, actualBlockAccepted, "Block %s should be accepted=%t but is %t", blockID, blockExpectedAccepted, actualBlockAccepted)
	}
}

func (n *Node) AssertEqualChainsAtLeastAtEpoch(index epoch.Index, other *Node) {
	lastCommitment := n.Protocol.Engine().Storage.Settings.LatestCommitment()
	otherLastCommitment := other.Protocol.Engine().Storage.Settings.LatestCommitment()

	require.GreaterOrEqual(n.Testing, lastCommitment.Index(), index)
	require.GreaterOrEqual(n.Testing, otherLastCommitment.Index(), index)

	oldestIndex := lo.Min(lastCommitment.Index(), otherLastCommitment.Index())
	require.Equal(n.Testing, lo.PanicOnErr(n.Protocol.Engine().Storage.Commitments.Load(oldestIndex)), lo.PanicOnErr(other.Protocol.Engine().Storage.Commitments.Load(oldestIndex)))
}
