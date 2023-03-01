package network

import (
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
)

// region MockedNetwork ////////////////////////////////////////////////////////////////////////////////////////////////

const mainPartition = "main"

type MockedNetwork struct {
	dispatchersByPartition map[string]map[identity.ID]*MockedEndpoint
	dispatchersMutex       sync.RWMutex
}

func NewMockedNetwork() (mockedNetwork *MockedNetwork) {
	return &MockedNetwork{
		dispatchersByPartition: map[string]map[identity.ID]*MockedEndpoint{
			mainPartition: make(map[identity.ID]*MockedEndpoint),
		},
	}
}

func (m *MockedNetwork) Join(endpointID identity.ID, partition ...string) (endpoint *MockedEndpoint) {
	m.dispatchersMutex.Lock()
	defer m.dispatchersMutex.Unlock()

	partitionID := mainPartition
	if len(partition) > 0 {
		partitionID = partition[0]
	}
	endpoint = NewMockedEndpoint(endpointID, m, partitionID)

	dispatchers, exists := m.dispatchersByPartition[partitionID]
	if !exists {
		dispatchers = make(map[identity.ID]*MockedEndpoint)
		m.dispatchersByPartition[partitionID] = dispatchers
	}
	dispatchers[endpointID] = endpoint

	return
}

func (m *MockedNetwork) MergePartitionsToMain(partitions ...string) {
	m.dispatchersMutex.Lock()
	defer m.dispatchersMutex.Unlock()

	switch {
	case len(partitions) == 0:
		// Merge all partitions to main
		for partitionID := range m.dispatchersByPartition {
			if partitionID != mainPartition {
				m.mergePartition(partitionID)
			}
		}
	default:
		for _, partitionID := range partitions {
			m.mergePartition(partitionID)
		}
	}
}

func (m *MockedNetwork) mergePartition(partitionID string) {
	for _, endpoint := range m.dispatchersByPartition[partitionID] {
		endpoint.partition = mainPartition
		m.dispatchersByPartition[mainPartition][endpoint.id] = endpoint
	}
	delete(m.dispatchersByPartition, partitionID)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedEndpoint ///////////////////////////////////////////////////////////////////////////////////////////////

type MockedEndpoint struct {
	id            identity.ID
	network       *MockedNetwork
	partition     string
	handlers      map[string]func(identity.ID, proto.Message) error
	handlersMutex sync.RWMutex
}

func NewMockedEndpoint(id identity.ID, network *MockedNetwork, partition string) (newMockedNetwork *MockedEndpoint) {
	return &MockedEndpoint{
		id:        id,
		network:   network,
		partition: partition,
		handlers:  make(map[string]func(identity.ID, proto.Message) error),
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
		to = lo.Keys(m.network.dispatchersByPartition[m.partition])
	}

	for _, id := range to {
		if id == m.id {
			continue
		}

		if dispatcher, exists := m.network.dispatchersByPartition[m.partition][id]; exists {
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
