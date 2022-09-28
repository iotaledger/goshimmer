package network

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"google.golang.org/protobuf/proto"
)

// region MockedNetwork ////////////////////////////////////////////////////////////////////////////////////////////////

type MockedNetwork struct {
	dispatchers map[identity.ID]*MockedDispatcher
}

func NewMockedNetwork() (mockedNetwork *MockedNetwork) {
	return &MockedNetwork{
		dispatchers: make(map[identity.ID]*MockedDispatcher),
	}
}

func (m *MockedNetwork) CreateDispatcher() (dispatcher *MockedDispatcher) {
	id := identity.GenerateIdentity().ID()
	m.dispatchers[id] = NewMockedDispatcher(id, m)

	return m.dispatchers[id]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedDispatcher /////////////////////////////////////////////////////////////////////////////////////////////

type MockedDispatcher struct {
	id       identity.ID
	network  *MockedNetwork
	handlers map[string]func(identity.ID, proto.Message) error
}

func NewMockedDispatcher(id identity.ID, network *MockedNetwork) (newMockedNetwork *MockedDispatcher) {
	return &MockedDispatcher{
		id:       id,
		network:  network,
		handlers: make(map[string]func(identity.ID, proto.Message) error),
	}
}

func (m *MockedDispatcher) RegisterProtocol(protocolID string, newMessage func() proto.Message, handler func(identity.ID, proto.Message) error) {
	m.handlers[protocolID] = handler
}

func (m *MockedDispatcher) UnregisterProtocol(protocolID string) {
	delete(m.handlers, protocolID)
}

func (m *MockedDispatcher) Send(packet proto.Message, protocolID string, to ...identity.ID) []identity.ID {
	if len(to) == 0 {
		to = lo.Keys(m.network.dispatchers)
	}

	for _, id := range to {
		m.network.dispatchers[id].handlers[protocolID](m.id, packet)
	}

	return nil
}

var _ Dispatcher = &MockedDispatcher{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
