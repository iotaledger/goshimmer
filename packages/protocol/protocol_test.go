package protocol

import (
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/tools/genesis-snapshot/snapshotcreator"
)

func TestProtocol(t *testing.T) {
	debug.SetEnabled(true)

	testNetwork := network.NewMockedNetwork()

	diskUtil1 := diskutil.New(t.TempDir())
	diskUtil2 := diskutil.New(t.TempDir())

	require.NoError(t, snapshotcreator.CreateSnapshot(diskUtil1.Path("snapshot.bin"), 100, make([]byte, 32, 32), map[identity.ID]uint64{
		identity.GenerateIdentity().ID(): 100,
	}))

	require.NoError(t, snapshotcreator.CreateSnapshot(diskUtil2.Path("snapshot.bin"), 100, make([]byte, 32, 32), map[identity.ID]uint64{
		identity.GenerateIdentity().ID(): 100,
	}))

	protocol1 := New(testNetwork.CreateDispatcher(), WithBaseDirectory(diskUtil1.Path()))
	_ = New(testNetwork.CreateDispatcher(), WithBaseDirectory(diskUtil2.Path()))

	tf := engine.NewTestFramework(t, engine.WithEngine(protocol1.activeInstance))
	tf.Tangle.CreateBlock("A", models.WithStrongParents(tf.Tangle.BlockIDs("Genesis")))
	tf.Tangle.IssueBlocks("A")
	event.Loop.WaitUntilAllTasksProcessed()
}
