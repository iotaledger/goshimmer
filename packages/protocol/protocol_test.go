package protocol

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/tools/genesis-snapshot/snapshotcreator"
)

func init() {
	if err := logger.InitGlobalLogger(configuration.New()); err != nil {
		panic(err)
	}
}

func TestProtocol(t *testing.T) {
	log := logger.NewLogger(t.Name())
	diskUtil := diskutil.New(t.TempDir())

	require.NoError(t, snapshotcreator.CreateSnapshot(diskUtil.Path("snapshot.bin"), 100, make([]byte, 32, 32), map[identity.ID]uint64{
		identity.GenerateIdentity().ID(): 100,
	}))

	protocol := New(nil, log, WithBaseDirectory(diskUtil.Path()))

	fmt.Println(protocol)

	debug.SetEnabled(true)

	tf := engine.NewTestFramework(t, engine.WithEngine(protocol.activeInstance.Engine))
	tf.Tangle.CreateBlock("A", models.WithStrongParents(tf.Tangle.BlockIDs("Genesis")))
	tf.Tangle.IssueBlocks("A")
	event.Loop.WaitUntilAllTasksProcessed()
}
