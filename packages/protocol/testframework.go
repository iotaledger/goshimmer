package protocol

import (
	"testing"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/snapshot/creator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Network  network.Interface
	Protocol *Protocol

	Engine *engine.TestFramework

	test *testing.T

	optsProtocolOptions []options.Option[Protocol]
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	_ = logger.InitGlobalLogger(configuration.New())

	return options.Apply(&TestFramework{
		Network: NewMockedNetwork(),

		test: test,
	}, opts, func(t *TestFramework) {
		diskUtil := diskutil.New(test.TempDir())

		snapshotPath := diskUtil.Path("snapshot.bin")
		require.NoError(test, creator.CreateSnapshot(snapshotPath, 100, make([]byte, 32, 32), map[identity.ID]uint64{
			identity.GenerateIdentity().ID(): 100,
		}))

		t.Protocol = New(t.Network, logger.NewLogger(test.Name()), append(t.optsProtocolOptions, WithBaseDirectory(diskUtil.Path()), WithSnapshotPath(snapshotPath))...)
		t.Engine = engine.NewTestFramework(test, engine.WithEngine(t.Protocol.Engine()))

		t.Protocol.Run()
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedNetwork ////////////////////////////////////////////////////////////////////////////////////////////////

type MockedNetwork struct {
	events *network.Events
}

func NewMockedNetwork() (newMockedNetwork *MockedNetwork) {
	return &MockedNetwork{
		events: network.NewEvents(),
	}
}

func (m *MockedNetwork) Events() *network.Events {
	return m.events
}

func (m *MockedNetwork) SendBlock(block *models.Block, peers ...*peer.Peer) {
	// TODO implement me
}

func (m *MockedNetwork) RequestBlock(id models.BlockID, peers ...*peer.Peer) {
	// TODO implement me
}

var _ network.Interface = &MockedNetwork{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
