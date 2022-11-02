package protocol

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/snapshot/creator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
)

func TestProtocol(t *testing.T) {
	debug.SetEnabled(true)

	testNetwork := network.NewMockedNetwork()

	endpoint1 := testNetwork.Join(identity.GenerateIdentity().ID())
	diskUtil1 := diskutil.New(t.TempDir())

	s := storage.New(lo.PanicOnErr(os.MkdirTemp(os.TempDir(), "*")), DatabaseVersion)
	creator.CreateSnapshot(s, diskUtil1.Path("snapshot.bin"), 100, make([]byte, 32, 32), map[identity.ID]uint64{
		identity.GenerateIdentity().ID(): 100,
	})

	protocol1 := New(endpoint1, WithBaseDirectory(diskUtil1.Path()), WithSnapshotPath(diskUtil1.Path("snapshot.bin")))
	protocol1.Run()

	commitments := make(map[string]*commitment.Commitment)
	commitments["0"] = commitment.New(0, commitment.ID{}, types.Identifier{}, 0)
	commitments["1"] = commitment.New(1, commitments["0"].ID(), types.Identifier{1}, 0)
	commitments["2"] = commitment.New(2, commitments["1"].ID(), types.Identifier{2}, 0)
	commitments["3"] = commitment.New(3, commitments["2"].ID(), types.Identifier{3}, 0)

	protocol1.networkProtocol.Events.EpochCommitmentReceived.Trigger(&network.EpochCommitmentReceivedEvent{
		Commitment: commitments["1"],
		Neighbor:   identity.ID{},
	})

	protocol1.networkProtocol.Events.EpochCommitmentReceived.Trigger(&network.EpochCommitmentReceivedEvent{
		Commitment: commitments["2"],
		Neighbor:   identity.ID{},
	})

	protocol1.networkProtocol.Events.EpochCommitmentReceived.Trigger(&network.EpochCommitmentReceivedEvent{
		Commitment: commitments["3"],
		Neighbor:   identity.ID{},
	})

	endpoint2 := testNetwork.Join(identity.GenerateIdentity().ID())
	diskUtil2 := diskutil.New(t.TempDir())

	s2 := storage.New(lo.PanicOnErr(os.MkdirTemp(os.TempDir(), "*")), DatabaseVersion)
	creator.CreateSnapshot(s2, diskUtil2.Path("snapshot.bin"), 100, make([]byte, 32, 32), map[identity.ID]uint64{
		identity.GenerateIdentity().ID(): 100,
	})

	protocol2 := New(endpoint2, WithBaseDirectory(diskUtil2.Path()), WithSnapshotPath(diskUtil2.Path("snapshot.bin")))
	protocol2.Run()

	protocol2.chainManager.Events.CommitmentMissing.Hook(event.NewClosure(func(id commitment.ID) {
		fmt.Println("MISSING", id)
	}))
	protocol2.chainManager.Events.MissingCommitmentReceived.Hook(event.NewClosure(func(id commitment.ID) {
		fmt.Println("MISSING RECEIVED", id)
	}))

	protocol2.networkProtocol.Events.EpochCommitmentReceived.Trigger(&network.EpochCommitmentReceivedEvent{
		Commitment: commitments["3"],
		Neighbor:   identity.ID{},
	})

	tf1 := engine.NewTestFramework(t, engine.WithEngine(protocol1.Engine()))
	_ = engine.NewTestFramework(t, engine.WithEngine(protocol2.Engine()))

	tf1.Tangle.CreateBlock("A", models.WithStrongParents(tf1.Tangle.BlockIDs("Genesis")))
	tf1.Tangle.IssueBlocks("A")

	time.Sleep(4 * time.Second)

	event.Loop.WaitUntilAllTasksProcessed()
}
