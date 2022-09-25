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
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/tools/genesis-snapshot/snapshotcreator"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Network  network.Interface
	Protocol *Protocol

	test *testing.T

	optsProtocolOptions []options.Option[Protocol]
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	return options.Apply(&TestFramework{
		Network: NewMockedNetwork(),

		test: test,
	}, opts, func(t *TestFramework) {
		diskUtil := diskutil.New(test.TempDir())

		require.NoError(test, snapshotcreator.CreateSnapshot(diskUtil.Path("snapshot.bin"), 100, make([]byte, 32, 32), map[identity.ID]uint64{
			identity.GenerateIdentity().ID(): 100,
		}))

		t.Protocol = New(t.Network, logger.NewLogger(test.Name()), append(t.optsProtocolOptions, WithBaseDirectory(diskUtil.Path()))...)
	})
}

func init() {
	_ = logger.InitGlobalLogger(configuration.New())
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
	panic("implement me")
}

func (m *MockedNetwork) RequestBlock(id models.BlockID, peers ...*peer.Peer) {
	// TODO implement me
	panic("implement me")
}

var _ network.Interface = &MockedNetwork{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
