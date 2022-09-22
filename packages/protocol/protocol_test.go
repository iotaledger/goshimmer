package protocol

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/network"
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

	testNetwork := newMockedNetwork()

	protocol := New(testNetwork, log, WithBaseDirectory(diskUtil.Path()))

	fmt.Println(protocol)

	debug.SetEnabled(true)

	tf := engine.NewTestFramework(t, engine.WithEngine(protocol.activeInstance.Engine))
	tf.Tangle.CreateBlock("A", models.WithStrongParents(tf.Tangle.BlockIDs("Genesis")))
	tf.Tangle.IssueBlocks("A")
	event.Loop.WaitUntilAllTasksProcessed()
}

type mockedNetwork struct {
	events *network.Events
}

func newMockedNetwork() (newMockedNetwork *mockedNetwork) {
	return &mockedNetwork{
		events: network.NewEvents(),
	}
}

func (m *mockedNetwork) Events() *network.Events {
	return m.events
}

func (m *mockedNetwork) SendBlock(block *models.Block, peers ...*peer.Peer) {
	// TODO implement me
	panic("implement me")
}

func (m *mockedNetwork) RequestBlock(id models.BlockID, peers ...*peer.Peer) {
	// TODO implement me
	panic("implement me")
}

var _ network.Interface = &mockedNetwork{}
