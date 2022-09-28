package protocol

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/snapshot/creator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func TestProtocol(t *testing.T) {
	debug.SetEnabled(true)

	testNetwork := network.NewMockedNetwork()

	endpoint1 := testNetwork.Join(identity.GenerateIdentity().ID())
	endpoint2 := testNetwork.Join(identity.GenerateIdentity().ID())

	diskUtil1 := diskutil.New(t.TempDir())
	diskUtil2 := diskutil.New(t.TempDir())

	require.NoError(t, creator.CreateSnapshot(diskUtil1.Path("snapshot.bin"), 100, make([]byte, 32, 32), map[identity.ID]uint64{
		identity.GenerateIdentity().ID(): 100,
	}))

	require.NoError(t, creator.CreateSnapshot(diskUtil2.Path("snapshot.bin"), 100, make([]byte, 32, 32), map[identity.ID]uint64{
		identity.GenerateIdentity().ID(): 100,
	}))

	protocol1 := New(endpoint1, WithBaseDirectory(diskUtil1.Path()), WithSnapshotPath(diskUtil1.Path("snapshot.bin")))
	protocol2 := New(endpoint2, WithBaseDirectory(diskUtil2.Path()), WithSnapshotPath(diskUtil2.Path("snapshot.bin")))

	protocol1.Run()
	protocol2.Run()

	tf1 := engine.NewTestFramework(t, engine.WithEngine(protocol1.activeInstance))
	_ = engine.NewTestFramework(t, engine.WithEngine(protocol2.activeInstance))

	tf1.Tangle.CreateBlock("A", models.WithStrongParents(tf1.Tangle.BlockIDs("Genesis")))
	tf1.Tangle.IssueBlocks("A")

	time.Sleep(4 * time.Second)

	event.Loop.WaitUntilAllTasksProcessed()
}
