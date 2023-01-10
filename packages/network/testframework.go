package network

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"google.golang.org/protobuf/proto"
)

// region MockedNetwork ////////////////////////////////////////////////////////////////////////////////////////////////

type MockedNetwork struct {
	dispatchers      map[identity.ID]*MockedEndpoint
	dispatchersMutex sync.RWMutex
}

func NewMockedNetwork() (mockedNetwork *MockedNetwork) {
	return &MockedNetwork{
		dispatchers: make(map[identity.ID]*MockedEndpoint),
	}
}

func (m *MockedNetwork) Join(identity identity.ID) (endpoint *MockedEndpoint) {
	m.dispatchersMutex.Lock()
	defer m.dispatchersMutex.Unlock()

	endpoint = NewMockedEndpoint(identity, m)

	m.dispatchers[identity] = endpoint

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedEndpoint ///////////////////////////////////////////////////////////////////////////////////////////////

type MockedEndpoint struct {
	id            identity.ID
	network       *MockedNetwork
	handlers      map[string]func(identity.ID, proto.Message) error
	handlersMutex sync.RWMutex
}

func NewMockedEndpoint(id identity.ID, network *MockedNetwork) (newMockedNetwork *MockedEndpoint) {
	return &MockedEndpoint{
		id:       id,
		network:  network,
		handlers: make(map[string]func(identity.ID, proto.Message) error),
	}
}

func (m *MockedEndpoint) RegisterProtocol(protocolID string, newMessage func() proto.Message, handler func(identity.ID, proto.Message) error) {
	m.handlersMutex.Lock()
	defer m.handlersMutex.Unlock()

	m.handlers[protocolID] = handler
}

func (m *MockedEndpoint) UnregisterProtocol(protocolID string) {
	m.handlersMutex.Lock()
	defer m.handlersMutex.Unlock()

	delete(m.handlers, protocolID)
}

func (m *MockedEndpoint) Send(packet proto.Message, protocolID string, to ...identity.ID) {
	m.network.dispatchersMutex.RLock()
	defer m.network.dispatchersMutex.RUnlock()

	if len(to) == 0 {
		to = lo.Keys(m.network.dispatchers)
	}

	for _, id := range to {
		if id == m.id {
			continue
		}

		if dispatcher, exists := m.network.dispatchers[id]; exists {
			if protocolHandler, exists := dispatcher.handler(protocolID); exists {
				if err := protocolHandler(m.id, packet); err != nil {
					fmt.Println("ERROR: ", err)
				}
			}
		}
	}
}

func (m *MockedEndpoint) handler(protocolID string) (handler func(identity.ID, proto.Message) error, exists bool) {
	m.handlersMutex.RLock()
	defer m.handlersMutex.RUnlock()

	handler, exists = m.handlers[protocolID]

	return
}

var _ Endpoint = &MockedEndpoint{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
