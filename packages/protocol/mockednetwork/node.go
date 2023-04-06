package mockednetwork

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/filter"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization/slotnotarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/inmemorytangle"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

type Node struct {
	Testing  *testing.T
	Name     string
	BaseDir  *utils.Directory
	KeyPair  ed25519.KeyPair
	Endpoint *network.MockedEndpoint
	Workers  *workerpool.Group
	Protocol *protocol.Protocol

	tf    *engine.TestFramework
	mutex sync.RWMutex
}

func NewNode(t *testing.T, keyPair ed25519.KeyPair, network *network.MockedNetwork, partition string, snapshotPath string, ledgerProvider module.Provider[*engine.Engine, ledger.Ledger]) *Node {
	id := identity.New(keyPair.PublicKey)

	node := &Node{
		Testing:  t,
		Workers:  workerpool.NewGroup(id.ID().String()),
		Name:     id.ID().String(),
		BaseDir:  utils.NewDirectory(t.TempDir()),
		KeyPair:  keyPair,
		Endpoint: network.Join(id.ID(), partition),
	}

	node.Protocol = protocol.New(node.Workers.CreateGroup("Protocol"),
		node.Endpoint,
		protocol.WithBaseDirectory(node.BaseDir.Path()),
		protocol.WithSnapshotPath(snapshotPath),
		protocol.WithStorageDatabaseManagerOptions(database.WithDBProvider(database.NewDB)),
		protocol.WithLedgerProvider(ledgerProvider),
		protocol.WithTangleProvider(
			inmemorytangle.NewProvider(
				inmemorytangle.WithBookerProvider(
					markerbooker.NewProvider(
						markerbooker.WithMarkerManagerOptions(
							markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
						),
					),
				),
			),
		),
		protocol.WithNotarizationProvider(
			slotnotarization.NewProvider(
				slotnotarization.WithMinCommittableSlotAge(1),
			),
		),
	)
	node.Protocol.Run()

	mainEngine := node.Protocol.MainEngineInstance()
	node.tf = engine.NewTestFramework(t, node.Workers.CreateGroup(fmt.Sprintf("EngineTestFramework-%s", mainEngine.Name()[:8])), mainEngine)

	node.Protocol.Events.CandidateEngineActivated.Hook(func(candidateEngine *engine.Engine) {
		node.mutex.Lock()
		defer node.mutex.Unlock()

		node.tf = engine.NewTestFramework(node.Testing, node.Workers.CreateGroup(fmt.Sprintf("EngineTestFramework-%s", candidateEngine.Name()[:8])), candidateEngine)
	})

	return node
}

func NewNodeFromDisk(t *testing.T, keyPair ed25519.KeyPair, network *network.MockedNetwork, partition string, baseDir string) *Node {
	id := identity.New(keyPair.PublicKey)

	node := &Node{
		Testing:  t,
		Workers:  workerpool.NewGroup(id.ID().String()),
		Name:     id.ID().String(),
		BaseDir:  utils.NewDirectory(baseDir),
		KeyPair:  keyPair,
		Endpoint: network.Join(id.ID(), partition),
	}

	node.Protocol = protocol.New(node.Workers.CreateGroup("Protocol"),
		node.Endpoint,
		protocol.WithBaseDirectory(node.BaseDir.Path()),
		protocol.WithStorageDatabaseManagerOptions(database.WithDBProvider(database.NewDB)),
		protocol.WithTangleProvider(
			inmemorytangle.NewProvider(
				inmemorytangle.WithBookerProvider(
					markerbooker.NewProvider(
						markerbooker.WithMarkerManagerOptions(
							markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
						),
					),
				),
			),
		),
		protocol.WithNotarizationProvider(
			slotnotarization.NewProvider(
				slotnotarization.WithMinCommittableSlotAge(1),
			),
		),
	)
	node.Protocol.Run()

	mainEngine := node.Protocol.MainEngineInstance()
	node.tf = engine.NewTestFramework(t, node.Workers.CreateGroup(fmt.Sprintf("EngineTestFramework-%s", mainEngine.Name()[:8])), mainEngine)

	node.Protocol.Events.CandidateEngineActivated.Hook(func(candidateEngine *engine.Engine) {
		node.mutex.Lock()
		defer node.mutex.Unlock()

		node.tf = engine.NewTestFramework(node.Testing, node.Workers.CreateGroup(fmt.Sprintf("EngineTestFramework-%s", candidateEngine.Name()[:8])), candidateEngine)
	})

	return node
}

func (n *Node) EngineTestFramework() *engine.TestFramework {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	return n.tf
}

func (n *Node) HookLogging(includeMainEngine bool) {
	events := n.Protocol.Events

	if includeMainEngine {
		n.attachEngineLogs(n.Protocol.MainEngineInstance())
	}

	events.CandidateEngineActivated.Hook(func(candidateEngine *engine.Engine) {
		fmt.Printf("%s > CandidateEngineActivated: latest commitment %s %s\n", n.Name, candidateEngine.Storage.Settings.LatestCommitment().ID(), candidateEngine.Storage.Settings.LatestCommitment())
		fmt.Printf("==================\nACTIVATE %s\n==================\n", n.Name)
		n.attachEngineLogs(candidateEngine)
	})

	events.MainEngineSwitched.Hook(func(engine *engine.Engine) {
		fmt.Printf("%s > MainEngineSwitched: latest commitment %s %s\n", n.Name, engine.Storage.Settings.LatestCommitment().ID(), engine.Storage.Settings.LatestCommitment())
		fmt.Printf("================\nSWITCH %s\n================\n", n.Name)
	})

	events.CongestionControl.Scheduler.BlockScheduled.Hook(func(block *scheduler.Block) {
		fmt.Printf("%s > CongestionControl.Scheduler.BlockScheduled: %s\n", n.Name, block.ID())
	})

	events.CongestionControl.Scheduler.BlockDropped.Hook(func(block *scheduler.Block) {
		fmt.Printf("%s > CongestionControl.Scheduler.BlockDropped: %s\n", n.Name, block.ID())
	})

	events.CongestionControl.Scheduler.BlockSubmitted.Hook(func(block *scheduler.Block) {
		fmt.Printf("%s > CongestionControl.Scheduler.BlockSubmitted: %s\n", n.Name, block.ID())
	})

	events.CongestionControl.Scheduler.BlockSkipped.Hook(func(block *scheduler.Block) {
		fmt.Printf("%s > CongestionControl.Scheduler.BlockSkipped: %s\n", n.Name, block.ID())
	})

	events.ChainManager.ForkDetected.Hook(func(fork *chainmanager.Fork) {
		fmt.Printf("%s > ChainManager.ForkDetected: %s with forking point %s received from %s\n", n.Name, fork.Commitment.ID(), fork.ForkingPoint.ID(), fork.Source)
		fmt.Printf("----------------------\nForkDetected %s\n----------------------\n", n.Name)
	})

	events.Error.Hook(func(err error) {
		fmt.Printf("%s > Error: %s\n", n.Name, err.Error())
	})

	events.Network.BlockReceived.Hook(func(event *network.BlockReceivedEvent) {
		fmt.Printf("%s > Network.BlockReceived: from %s %s - %d\n", n.Name, event.Source, event.Block.ID(), event.Block.ID().Index())
	})

	events.Network.BlockRequestReceived.Hook(func(event *network.BlockRequestReceivedEvent) {
		fmt.Printf("%s > Network.BlockRequestReceived: from %s %s\n", n.Name, event.Source, event.BlockID)
	})

	events.Network.AttestationsReceived.Hook(func(event *network.AttestationsReceivedEvent) {
		fmt.Printf("%s > Network.AttestationsReceived: from %s for %s\n", n.Name, event.Source, event.Commitment.ID())
	})

	events.Network.AttestationsRequestReceived.Hook(func(event *network.AttestationsRequestReceivedEvent) {
		fmt.Printf("%s > Network.AttestationsRequestReceived: from %s %s -> %d\n", n.Name, event.Source, event.Commitment.ID(), event.EndIndex)
	})

	events.Network.SlotCommitmentReceived.Hook(func(event *network.SlotCommitmentReceivedEvent) {
		fmt.Printf("%s > Network.SlotCommitmentReceived: from %s %s\n", n.Name, event.Source, event.Commitment.ID())
	})

	events.Network.SlotCommitmentRequestReceived.Hook(func(event *network.SlotCommitmentRequestReceivedEvent) {
		fmt.Printf("%s > Network.SlotCommitmentRequestReceived: from %s %s\n", n.Name, event.Source, event.CommitmentID)
	})

	events.Network.Error.Hook(func(event *network.ErrorEvent) {
		fmt.Printf("%s > Network.Error: from %s %s\n", n.Name, event.Source, event.Error.Error())
	})
}

func (n *Node) attachEngineLogs(instance *engine.Engine) {
	engineName := fmt.Sprintf("%s - %s", lo.Cond(n.Protocol.Engine() != instance, "Candidate", "Main"), instance.Name()[:8])
	events := instance.Events

	events.Tangle.BlockDAG.BlockAttached.Hook(func(block *blockdag.Block) {
		fmt.Printf("%s > [%s] BlockDAG.BlockAttached: %s\n", n.Name, engineName, block.ID())
	})

	events.Tangle.BlockDAG.BlockSolid.Hook(func(block *blockdag.Block) {
		fmt.Printf("%s > [%s] BlockDAG.BlockSolid: %s\n", n.Name, engineName, block.ID())
	})

	events.Tangle.BlockDAG.BlockInvalid.Hook(func(event *blockdag.BlockInvalidEvent) {
		fmt.Printf("%s > [%s] BlockDAG.BlockInvalid: %s - %s\n", n.Name, engineName, event.Block.ID(), event.Reason.Error())
	})

	events.Tangle.BlockDAG.BlockMissing.Hook(func(block *blockdag.Block) {
		fmt.Printf("%s > [%s] BlockDAG.BlockMissing: %s\n", n.Name, engineName, block.ID())
	})

	events.Tangle.BlockDAG.MissingBlockAttached.Hook(func(block *blockdag.Block) {
		fmt.Printf("%s > [%s] BlockDAG.MissingBlockAttached: %s\n", n.Name, engineName, block.ID())
	})

	events.Tangle.BlockDAG.BlockOrphaned.Hook(func(block *blockdag.Block) {
		fmt.Printf("%s > [%s] BlockDAG.BlockOrphaned: %s\n", n.Name, engineName, block.ID())
	})

	events.Tangle.BlockDAG.BlockUnorphaned.Hook(func(block *blockdag.Block) {
		fmt.Printf("%s > [%s] BlockDAG.BlockUnorphaned: %s\n", n.Name, engineName, block.ID())
	})

	events.Tangle.Booker.BlockBooked.Hook(func(evt *booker.BlockBookedEvent) {
		fmt.Printf("%s > [%s] Booker.BlockBooked: %s\n", n.Name, engineName, evt.Block.ID())
	})

	events.Tangle.Booker.SequenceTracker.VotersUpdated.Hook(func(event *sequencetracker.VoterUpdatedEvent) {
		fmt.Printf("%s > [%s] Tangle.VirtualVoting.SequenceTracker.VotersUpdated: %s %s %d -> %d\n", n.Name, engineName, event.Voter, event.SequenceID, event.PrevMaxSupportedIndex, event.NewMaxSupportedIndex)
	})

	events.Clock.AcceptedTimeUpdated.Hook(func(newTime time.Time) {
		fmt.Printf("%s > [%s] Clock.AcceptedTimeUpdated: %s\n", n.Name, engineName, newTime)
	})

	events.Filter.BlockAllowed.Hook(func(block *models.Block) {
		fmt.Printf("%s > [%s] Filter.BlockAllowed: %s\n", n.Name, engineName, block.ID())
	})

	events.Filter.BlockFiltered.Hook(func(event *filter.BlockFilteredEvent) {
		fmt.Printf("%s > [%s] Filter.BlockFiltered: %s - %s\n", n.Name, engineName, event.Block.ID(), event.Reason.Error())
		n.Testing.Fatal("no blocks should be filtered")
	})

	events.BlockRequester.Tick.Hook(func(blockID models.BlockID) {
		fmt.Printf("%s > [%s] BlockRequester.Tick: %s\n", n.Name, engineName, blockID)
	})

	events.BlockProcessed.Hook(func(blockID models.BlockID) {
		fmt.Printf("%s > [%s] Engine.BlockProcessed: %s\n", n.Name, engineName, blockID)
	})

	events.Error.Hook(func(err error) {
		fmt.Printf("%s > [%s] Engine.Error: %s\n", n.Name, engineName, err.Error())
	})

	events.Notarization.SlotCommitted.Hook(func(details *notarization.SlotCommittedDetails) {
		fmt.Printf("%s > [%s] NotarizationManager.SlotCommitted: %s %s\n", n.Name, engineName, details.Commitment.ID(), details.Commitment)
	})

	events.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
		fmt.Printf("%s > [%s] Consensus.BlockGadget.BlockAccepted: %s %s\n", n.Name, engineName, block.ID(), block.Commitment().ID())
	})

	events.Consensus.BlockGadget.BlockConfirmed.Hook(func(block *blockgadget.Block) {
		fmt.Printf("%s > [%s] Consensus.BlockGadget.BlockConfirmed: %s %s\n", n.Name, engineName, block.ID(), block.Commitment().ID())
	})

	events.Consensus.SlotGadget.SlotConfirmed.Hook(func(slotIndex slot.Index) {
		fmt.Printf("%s > [%s] Consensus.SlotGadget.SlotConfirmed: %s\n", n.Name, engineName, slotIndex)
	})
}

func (n *Node) Wait() {
	n.Workers.WaitChildren()
}

func (n *Node) IssueBlockAtSlot(alias string, slotIndex slot.Index, parents ...models.BlockID) *models.Block {
	tf := n.EngineTestFramework()

	issuingTime := time.Unix(tf.SlotTimeProvider().GenesisUnixTime()+int64(slotIndex-1)*tf.SlotTimeProvider().Duration(), 0)
	require.True(n.Testing, issuingTime.Before(time.Now()), "issued block is in the current or future slot")
	tf.BlockDAG.CreateAndSignBlock(alias, &n.KeyPair,
		models.WithStrongParents(models.NewBlockIDs(parents...)),
		models.WithIssuingTime(issuingTime),
		models.WithCommitment(n.Protocol.Engine().Storage.Settings.LatestCommitment()),
	)
	tf.BlockDAG.IssueBlocks(alias)
	fmt.Println(n.Name, alias, n.Workers)
	block := tf.BlockDAG.Block(alias)
	fmt.Printf("%s > IssueBlockAtSlot: %s with %s\n", n.Name, block.ID(), block.Commitment().ID())
	return block
}

func (n *Node) IssueBlock(alias string, parents ...models.BlockID) *models.Block {
	tf := n.EngineTestFramework()

	tf.BlockDAG.CreateAndSignBlock(alias, &n.KeyPair,
		models.WithStrongParents(models.NewBlockIDs(parents...)),
		models.WithCommitment(n.Protocol.Engine().Storage.Settings.LatestCommitment()),
	)
	tf.BlockDAG.IssueBlocks(alias)
	return tf.BlockDAG.Block(alias)
}

func (n *Node) IssueActivity(duration time.Duration, wg *sync.WaitGroup) {
	go func() {
		defer wg.Done()

		start := time.Now()
		fmt.Println(n.Name, "> Starting activity")
		var counter int
		for {
			if tips := n.Protocol.TipManager.Tips(1); len(tips) > 0 {
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
			} else {
				fmt.Println(n.Name, "> Skipped activity due lack of strong parents")
			}
		}
	}()
}

func (n *Node) issueActivityBlock(alias string, parents ...models.BlockID) bool {
	if !n.Protocol.Engine().WasStopped() {
		tf := n.EngineTestFramework()

		tf.BlockDAG.CreateAndSignBlock(alias, &n.KeyPair,
			models.WithStrongParents(models.NewBlockIDs(parents...)),
			models.WithCommitment(n.Protocol.Engine().Storage.Settings.LatestCommitment()),
		)
		tf.BlockDAG.IssueBlocks(alias)

		return true
	}
	return false
}

func (n *Node) ValidateAcceptedBlocks(expectedAcceptedBlocks map[models.BlockID]bool) {
	for blockID, blockExpectedAccepted := range expectedAcceptedBlocks {
		actualBlockAccepted := n.Protocol.Engine().Consensus.BlockGadget().IsBlockAccepted(blockID)
		require.Equal(n.Testing, blockExpectedAccepted, actualBlockAccepted, "Block %s should be accepted=%t but is %t", blockID, blockExpectedAccepted, actualBlockAccepted)
	}
}

func (n *Node) AssertEqualChainsAtLeastAtSlot(index slot.Index, other *Node) {
	lastCommitment := n.Protocol.Engine().Storage.Settings.LatestCommitment()
	otherLastCommitment := other.Protocol.Engine().Storage.Settings.LatestCommitment()

	require.GreaterOrEqual(n.Testing, lastCommitment.Index(), index)
	require.GreaterOrEqual(n.Testing, otherLastCommitment.Index(), index)

	oldestIndex := lo.Min(lastCommitment.Index(), otherLastCommitment.Index())
	require.Equal(n.Testing, lo.PanicOnErr(n.Protocol.Engine().Storage.Commitments.Load(oldestIndex)), lo.PanicOnErr(other.Protocol.Engine().Storage.Commitments.Load(oldestIndex)))
}
