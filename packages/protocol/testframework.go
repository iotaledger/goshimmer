package protocol

import (
	"testing"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/tools/genesis-snapshot/snapshotcreator"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Network  network.Network
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

func (m *MockedNetwork) RegisterProtocol(protocolID string, newMessage func() proto.Message, handler func(identity.ID, proto.Message) error) {
}

func (m *MockedNetwork) UnregisterProtocol(protocolID string) {
}

func (m *MockedNetwork) Send(packet proto.Message, protocolID string, to ...identity.ID) []identity.ID {
	return nil
}

var _ network.Network = &MockedNetwork{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
