package network

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"google.golang.org/protobuf/proto"
)

// region MockedNetwork ////////////////////////////////////////////////////////////////////////////////////////////////

type MockedNetwork struct {
	dispatchers map[identity.ID]*MockedEndpoint
}

func NewMockedNetwork() (mockedNetwork *MockedNetwork) {
	return &MockedNetwork{
		dispatchers: make(map[identity.ID]*MockedEndpoint),
	}
}

func (m *MockedNetwork) Join(identity identity.ID) (endpoint *MockedEndpoint) {
	endpoint = NewMockedEndpoint(identity, m)

	m.dispatchers[identity] = endpoint

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedEndpoint ///////////////////////////////////////////////////////////////////////////////////////////////

type MockedEndpoint struct {
	id       identity.ID
	network  *MockedNetwork
	handlers map[string]func(identity.ID, proto.Message) error
}

func NewMockedEndpoint(id identity.ID, network *MockedNetwork) (newMockedNetwork *MockedEndpoint) {
	return &MockedEndpoint{
		id:       id,
		network:  network,
		handlers: make(map[string]func(identity.ID, proto.Message) error),
	}
}

func (m *MockedEndpoint) RegisterProtocol(protocolID string, newMessage func() proto.Message, handler func(identity.ID, proto.Message) error) {
	m.handlers[protocolID] = handler
}

func (m *MockedEndpoint) UnregisterProtocol(protocolID string) {
	delete(m.handlers, protocolID)
}

func (m *MockedEndpoint) Send(packet proto.Message, protocolID string, to ...identity.ID) []identity.ID {
	if len(to) == 0 {
		to = lo.Keys(m.network.dispatchers)
	}

	for _, id := range to {
		if id == m.id {
			continue
		}

		m.network.dispatchers[id].handlers[protocolID](m.id, packet)
	}

	return nil
}

var _ Endpoint = &MockedEndpoint{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
