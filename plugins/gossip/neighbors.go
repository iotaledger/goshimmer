package gossip

import(
	"math"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/hive.go/events"
)

func configureNeighbors(plugin *node.Plugin) {
	Events.AddNeighbor.Attach(events.NewClosure(func(neighbor *Neighbor) {
		plugin.LogSuccess("new neighbor added " + neighbor.GetIdentity().StringIdentifier + "@" + neighbor.GetAddress().String() + ":" + strconv.Itoa(int(neighbor.GetPort())))
	}))

	Events.UpdateNeighbor.Attach(events.NewClosure(func(neighbor *Neighbor) {
		plugin.LogSuccess("existing neighbor updated " + neighbor.GetIdentity().StringIdentifier + "@" + neighbor.GetAddress().String() + ":" + strconv.Itoa(int(neighbor.GetPort())))
	}))

	Events.RemoveNeighbor.Attach(events.NewClosure(func(neighbor *Neighbor) {
		plugin.LogSuccess("existing neighbor removed " + neighbor.GetIdentity().StringIdentifier + "@" + neighbor.GetAddress().String() + ":" + strconv.Itoa(int(neighbor.GetPort())))
	}))
}

func runNeighbors(plugin *node.Plugin) {
	plugin.LogInfo("Starting Neighbor Connection Manager ...")

	neighborLock.RLock()
	for _, neighbor := range neighbors.GetMap() {
		manageConnection(plugin, neighbor)
	}
	neighborLock.RUnlock()

	Events.AddNeighbor.Attach(events.NewClosure(func(neighbor *Neighbor) {
		manageConnection(plugin, neighbor)
	}))

	plugin.LogSuccess("Starting Neighbor Connection Manager ... done")
}

func manageConnection(plugin *node.Plugin, neighbor *Neighbor) {
	daemon.BackgroundWorker("Connection Manager ("+neighbor.GetIdentity().StringIdentifier+")", func() {
		failedConnectionAttempts := 0

		for _, exists := neighbors.Load(neighbor.GetIdentity().StringIdentifier); exists && failedConnectionAttempts < CONNECTION_MAX_ATTEMPTS; {
			protocol, dialed, err := neighbor.Connect()
			if err != nil {
				failedConnectionAttempts++

				plugin.LogFailure("connection attempt [" + strconv.Itoa(int(failedConnectionAttempts)) + "/" + strconv.Itoa(CONNECTION_MAX_ATTEMPTS) + "] " + err.Error())

				if failedConnectionAttempts <= CONNECTION_MAX_ATTEMPTS {
					select {
					case <-daemon.ShutdownSignal:
						return

					case <-time.After(time.Duration(int(math.Pow(2, float64(failedConnectionAttempts-1)))) * CONNECTION_BASE_TIMEOUT):
						continue
					}
				}
			}

			failedConnectionAttempts = 0

			disconnectSignal := make(chan int, 1)
			protocol.Conn.Events.Close.Attach(events.NewClosure(func() {
				close(disconnectSignal)
			}))

			if dialed {
				go protocol.Init()
			}

			// wait for shutdown or
			select {
			case <-daemon.ShutdownSignal:
				return

			case <-disconnectSignal:
				continue
			}
		}

		RemoveNeighbor(neighbor.GetIdentity().StringIdentifier)
	})
}

type Neighbor struct {
	identity               *identity.Identity
	identityMutex          sync.RWMutex
	address                net.IP
	addressMutex           sync.RWMutex
	port                   uint16
	portMutex              sync.RWMutex
	initiatedProtocol      *protocol
	initiatedProtocolMutex sync.RWMutex
	acceptedProtocol       *protocol
	Events                 neighborEvents
	acceptedProtocolMutex  sync.RWMutex
}

func NewNeighbor(identity *identity.Identity, address net.IP, port uint16) *Neighbor {
	return &Neighbor{
		identity: identity,
		address:  address,
		port:     port,
		Events: neighborEvents{
			ProtocolConnectionEstablished: events.NewEvent(protocolCaller),
		},
	}
}

func (neighbor *Neighbor) GetIdentity() (result *identity.Identity) {
	neighbor.identityMutex.RLock()
	result = neighbor.identity
	neighbor.identityMutex.RUnlock()

	return result
}

func (neighbor *Neighbor) SetIdentity(identity *identity.Identity) {
	neighbor.identityMutex.Lock()
	neighbor.identity = identity
	neighbor.identityMutex.Unlock()
}

func (neighbor *Neighbor) GetAddress() (result net.IP) {
	neighbor.addressMutex.RLock()
	result = neighbor.address
	neighbor.addressMutex.RUnlock()

	return result
}

func (neighbor *Neighbor) SetAddress(address net.IP) {
	neighbor.addressMutex.Lock()
	neighbor.address = address
	neighbor.addressMutex.Unlock()
}

func (neighbor *Neighbor) GetPort() (result uint16) {
	neighbor.portMutex.RLock()
	result = neighbor.port
	neighbor.portMutex.RUnlock()

	return result
}

func (neighbor *Neighbor) SetPort(port uint16) {
	neighbor.portMutex.Lock()
	neighbor.port = port
	neighbor.portMutex.Unlock()
}

func (neighbor *Neighbor) GetInitiatedProtocol() (result *protocol) {
	neighbor.initiatedProtocolMutex.RLock()
	result = neighbor.initiatedProtocol
	neighbor.initiatedProtocolMutex.RUnlock()

	return result
}

func (neighbor *Neighbor) SetInitiatedProtocol(p *protocol) {
	neighbor.initiatedProtocolMutex.Lock()
	neighbor.initiatedProtocol = p
	neighbor.initiatedProtocolMutex.Unlock()
}

func (neighbor *Neighbor) GetAcceptedProtocol() (result *protocol) {
	neighbor.acceptedProtocolMutex.RLock()
	result = neighbor.acceptedProtocol
	neighbor.acceptedProtocolMutex.RUnlock()

	return result
}

func (neighbor *Neighbor) SetAcceptedProtocol(p *protocol) {
	neighbor.acceptedProtocolMutex.Lock()
	neighbor.acceptedProtocol = p
	neighbor.acceptedProtocolMutex.Unlock()
}

func UnmarshalPeer(data []byte) (*Neighbor, error) {
	return &Neighbor{}, nil
}

func (neighbor *Neighbor) Connect() (*protocol, bool, errors.IdentifiableError) {
	// return existing connections first
	if neighbor.GetInitiatedProtocol() != nil {
		return neighbor.GetInitiatedProtocol(), false, nil
	}

	// if we already have an accepted connection -> use it instead
	if neighbor.GetAcceptedProtocol() != nil {
		return neighbor.GetAcceptedProtocol(), false, nil
	}

	// otherwise try to dial
	conn, err := net.Dial("tcp", neighbor.GetAddress().String()+":"+strconv.Itoa(int(neighbor.GetPort())))
	if err != nil {
		return nil, false, ErrConnectionFailed.Derive(err, "error when connecting to neighbor "+
			neighbor.GetIdentity().StringIdentifier+"@"+neighbor.GetAddress().String()+":"+strconv.Itoa(int(neighbor.GetPort())))
	}

	neighbor.SetInitiatedProtocol(newProtocol(network.NewManagedConnection(conn)))

	neighbor.GetInitiatedProtocol().Conn.Events.Close.Attach(events.NewClosure(func() {
		neighbor.SetInitiatedProtocol(nil)
	}))

	// drop the "secondary" connection upon successful handshake
	neighbor.GetInitiatedProtocol().Events.HandshakeCompleted.Attach(events.NewClosure(func() {
		if accountability.OwnId().StringIdentifier <= neighbor.GetIdentity().StringIdentifier {
			var acceptedProtocolConn *network.ManagedConnection
			if neighbor.GetAcceptedProtocol() != nil {
				acceptedProtocolConn = neighbor.GetAcceptedProtocol().Conn
			}

			if acceptedProtocolConn != nil {
				_ = acceptedProtocolConn.Close()
			}
		}

		neighbor.Events.ProtocolConnectionEstablished.Trigger(neighbor.GetInitiatedProtocol())
	}))

	return neighbor.GetInitiatedProtocol(), true, nil
}

func (neighbor *Neighbor) Marshal() []byte {
	return nil
}

func (neighbor *Neighbor) Equals(other *Neighbor) bool {
	return neighbor.GetIdentity().StringIdentifier == other.GetIdentity().StringIdentifier &&
		neighbor.GetPort() == other.GetPort() && neighbor.GetAddress().String() == other.GetAddress().String()
}

func AddNeighbor(newNeighbor *Neighbor) {
	if neighbor, exists := neighbors.Load(newNeighbor.GetIdentity().StringIdentifier); !exists {
		neighbors.Store(newNeighbor.GetIdentity().StringIdentifier, newNeighbor)
		Events.AddNeighbor.Trigger(newNeighbor)
	} else {
		if !neighbor.Equals(newNeighbor) {
			neighbor.SetIdentity(newNeighbor.GetIdentity())
			neighbor.SetPort(newNeighbor.GetPort())
			neighbor.SetAddress(newNeighbor.GetAddress())

			Events.UpdateNeighbor.Trigger(neighbor)
		}
	}
}

func RemoveNeighbor(identifier string) {
	if neighbor, exists := neighbors.Delete(identifier); exists {
		Events.RemoveNeighbor.Trigger(neighbor)
	}
}

func GetNeighbor(identifier string) (*Neighbor, bool) {
	return neighbors.Load(identifier)
}

func GetNeighbors() map[string]*Neighbor {
	return neighbors.GetMap()
}

const (
	CONNECTION_MAX_ATTEMPTS = 5
	CONNECTION_BASE_TIMEOUT = 10 * time.Second
)

var neighbors = NewNeighborMap()

var neighborLock sync.RWMutex
