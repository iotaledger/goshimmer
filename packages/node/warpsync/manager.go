package warpsync

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	wp "github.com/iotaledger/goshimmer/packages/node/warpsync/warpsyncproto"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/logger"
	"google.golang.org/protobuf/proto"
)

const (
	protocolID = "warpsync/0.0.1"
)

// LoadBlockFunc defines a function that returns the block for the given id.
type LoadBlockFunc func(blockId tangle.BlockID) ([]byte, error)

// The Manager handles the connected neighbors.
type Manager struct {
	tangle     *tangle.Tangle
	p2pManager *p2p.Manager

	Events *Events

	log *logger.Logger

	stopMutex sync.RWMutex
	isStopped bool

	concurrency    int
	blockBatchSize int

	validationInProgress bool
	commitmentsChan      chan (*epoch.ECRecord)

	syncingInProgress       bool
	epochSyncStartChan      chan (*epochSyncStart)
	epochSyncBatchBlockChan chan (*epochSyncBatchBlock)
	epochSyncEndChan        chan (*epochSyncEnd)

	sync.RWMutex
}

// ManagerOption configures the Manager instance.
type ManagerOption func(m *Manager)

// NewManager creates a new Manager.
func NewManager(tangle *tangle.Tangle, p2pManager *p2p.Manager, log *logger.Logger, opts ...ManagerOption) *Manager {
	m := &Manager{
		tangle:                  tangle,
		p2pManager:              p2pManager,
		Events:                  newEvents(),
		log:                     log,
		commitmentsChan:         make(chan *epoch.ECRecord),
		epochSyncStartChan:      make(chan *epochSyncStart),
		epochSyncBatchBlockChan: make(chan *epochSyncBatchBlock),
		epochSyncEndChan:        make(chan *epochSyncEnd),
	}

	m.p2pManager.RegisterProtocol(protocolID, &p2p.ProtocolHandler{
		PacketFactory:      warpsyncPacketFactory,
		NegotiationSend:    sendNegotiationMessage,
		NegotiationReceive: receiveNegotiationMessage,
		PacketHandler:      m.handlePacket,
	})

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// WithConcurrency allows to set how many epochs can be requested at once.
func WithConcurrency(concurrency int) ManagerOption {
	return func(m *Manager) {
		m.concurrency = concurrency
	}
}

// WithBlockBatchSize allows to set the size of the block batch returned as part of epoch blocks response.
func WithBlockBatchSize(blockBatchSize int) ManagerOption {
	return func(m *Manager) {
		m.blockBatchSize = blockBatchSize
	}
}

// IsStopped returns true if the manager is stopped.
func (m *Manager) IsStopped() bool {
	return m.isStopped
}

// Stop stops the manager and closes all established connections.
func (m *Manager) Stop() {
	m.stopMutex.Lock()
	defer m.stopMutex.Unlock()

	if m.isStopped {
		return
	}

	close(m.commitmentsChan)
	close(m.epochSyncStartChan)
	close(m.epochSyncBatchBlockChan)
	close(m.epochSyncEndChan)

	m.isStopped = true
	m.p2pManager.UnregisterProtocol(protocolID)
}

func (m *Manager) handlePacket(nbr *p2p.Neighbor, packet proto.Message) error {
	wpPacket := packet.(*wp.Packet)
	switch packetBody := wpPacket.GetBody().(type) {
	case *wp.Packet_EpochBlocksRequest:
		if added := event.Loop.TrySubmit(func() { m.processEpochBlocksRequestPacket(packetBody, nbr) }); !added {
			return fmt.Errorf("WorkerPool full: packet block discarded")
		}
	case *wp.Packet_EpochBlocksStart:
		if added := event.Loop.TrySubmit(func() { m.processEpochBlocksStartPacket(packetBody, nbr) }); !added {
			return fmt.Errorf("WorkerPool full: packet block discarded")
		}
	case *wp.Packet_EpochBlocksBatch:
		if added := event.Loop.TrySubmit(func() { m.processEpochBlocksBatchPacket(packetBody, nbr) }); !added {
			return fmt.Errorf("WorkerPool full: packet block discarded")
		}
	case *wp.Packet_EpochBlocksEnd:
		if added := event.Loop.TrySubmit(func() { m.processEpochBlocksEndPacket(packetBody, nbr) }); !added {
			return fmt.Errorf("WorkerPool full: packet block discarded")
		}
	case *wp.Packet_EpochCommitmentRequest:
		if added := event.Loop.TrySubmit(func() { m.processEpochCommittmentRequestPacket(packetBody, nbr) }); !added {
			return fmt.Errorf("WorkerPool full: packet block discarded")
		}
	case *wp.Packet_EpochCommitment:
		if added := event.Loop.TrySubmit(func() { m.processEpochCommittmentPacket(packetBody, nbr) }); !added {
			return fmt.Errorf("WorkerPool full: packet block discarded")
		}
	default:
		return errors.Newf("unsupported packet; packet=%+v, packetBody=%T-%+v", wpPacket, packetBody, packetBody)
	}

	return nil
}

func warpsyncPacketFactory() proto.Message {
	return &wp.Packet{}
}

func sendNegotiationMessage(ps *p2p.PacketsStream) error {
	packet := &wp.Packet{Body: &wp.Packet_Negotiation{Negotiation: &wp.Negotiation{}}}
	return errors.WithStack(ps.WritePacket(packet))
}

func receiveNegotiationMessage(ps *p2p.PacketsStream) (err error) {
	packet := &wp.Packet{}
	if err := ps.ReadPacket(packet); err != nil {
		return errors.WithStack(err)
	}
	packetBody := packet.GetBody()
	if _, ok := packetBody.(*wp.Packet_Negotiation); !ok {
		return errors.Newf(
			"received packet isn't the negotiation packet; packet=%+v, packetBody=%T-%+v",
			packet, packetBody, packetBody,
		)
	}
	return nil
}
