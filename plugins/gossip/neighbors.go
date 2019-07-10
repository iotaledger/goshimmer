package gossip

import (
	"math"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/node"
)

func configureNeighbors(plugin *node.Plugin) {
	Events.AddNeighbor.Attach(events.NewClosure(func(neighbor *Neighbor) {
		plugin.LogSuccess("new neighbor added " + neighbor.Identity.StringIdentifier + "@" + neighbor.Address.String() + ":" + strconv.Itoa(int(neighbor.Port)))
	}))

	Events.UpdateNeighbor.Attach(events.NewClosure(func(neighbor *Neighbor) {
		plugin.LogSuccess("existing neighbor updated " + neighbor.Identity.StringIdentifier + "@" + neighbor.Address.String() + ":" + strconv.Itoa(int(neighbor.Port)))
	}))

	Events.RemoveNeighbor.Attach(events.NewClosure(func(neighbor *Neighbor) {
		plugin.LogSuccess("existing neighbor removed " + neighbor.Identity.StringIdentifier + "@" + neighbor.Address.String() + ":" + strconv.Itoa(int(neighbor.Port)))
	}))
}

func runNeighbors(plugin *node.Plugin) {
	plugin.LogInfo("Starting Neighbor Connection Manager ...")

	neighborLock.RLock()
	for _, neighbor := range GetNeighbors() {
		manageConnection(plugin, neighbor)
	}
	neighborLock.RUnlock()

	Events.AddNeighbor.Attach(events.NewClosure(func(neighbor *Neighbor) {
		manageConnection(plugin, neighbor)
	}))

	plugin.LogSuccess("Starting Neighbor Connection Manager ... done")
}

func manageConnection(plugin *node.Plugin, neighbor *Neighbor) {
	daemon.BackgroundWorker("Connection Manager ("+neighbor.Identity.StringIdentifier+")", func() {
		failedConnectionAttempts := 0

		for _, exists := GetNeighbor(neighbor.Identity.StringIdentifier); exists && failedConnectionAttempts < CONNECTION_MAX_ATTEMPTS; {
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

		RemoveNeighbor(neighbor.Identity.StringIdentifier)
	})
}

type Neighbor struct {
	Identity               *identity.Identity
	Address                net.IP
	Port                   uint16
	InitiatedProtocol      *protocol
	AcceptedProtocol       *protocol
	Events                 neighborEvents
	initiatedProtocolMutex sync.RWMutex
	acceptedProtocolMutex  sync.RWMutex
}

func NewNeighbor(identity *identity.Identity, address net.IP, port uint16) *Neighbor {
	return &Neighbor{
		Identity: identity,
		Address:  address,
		Port:     port,
		Events: neighborEvents{
			ProtocolConnectionEstablished: events.NewEvent(protocolCaller),
		},
	}
}

func UnmarshalPeer(data []byte) (*Neighbor, error) {
	return &Neighbor{}, nil
}

func (neighbor *Neighbor) Connect() (*protocol, bool, errors.IdentifiableError) {
	neighbor.initiatedProtocolMutex.Lock()
	defer neighbor.initiatedProtocolMutex.Unlock()

	// return existing connections first
	if neighbor.InitiatedProtocol != nil {
		return neighbor.InitiatedProtocol, false, nil
	}

	// if we already have an accepted connection -> use it instead
	if neighbor.AcceptedProtocol != nil {
		neighbor.acceptedProtocolMutex.RLock()
		if neighbor.AcceptedProtocol != nil {
			defer neighbor.acceptedProtocolMutex.RUnlock()

			return neighbor.AcceptedProtocol, false, nil
		}
		neighbor.acceptedProtocolMutex.RUnlock()
	}

	// otherwise try to dial
	conn, err := net.Dial("tcp", neighbor.Address.String()+":"+strconv.Itoa(int(neighbor.Port)))
	if err != nil {
		return nil, false, ErrConnectionFailed.Derive(err, "error when connecting to neighbor "+
			neighbor.Identity.StringIdentifier+"@"+neighbor.Address.String()+":"+strconv.Itoa(int(neighbor.Port)))
	}

	neighbor.InitiatedProtocol = newProtocol(network.NewManagedConnection(conn))

	neighbor.InitiatedProtocol.Conn.Events.Close.Attach(events.NewClosure(func() {
		neighbor.initiatedProtocolMutex.Lock()
		defer neighbor.initiatedProtocolMutex.Unlock()

		neighbor.InitiatedProtocol = nil
	}))

	// drop the "secondary" connection upon successful handshake
	neighbor.InitiatedProtocol.Events.HandshakeCompleted.Attach(events.NewClosure(func() {
		if accountability.OwnId().StringIdentifier <= neighbor.Identity.StringIdentifier {
			neighbor.acceptedProtocolMutex.Lock()
			var acceptedProtocolConn *network.ManagedConnection
			if neighbor.AcceptedProtocol != nil {
				acceptedProtocolConn = neighbor.AcceptedProtocol.Conn
			}
			neighbor.acceptedProtocolMutex.Unlock()

			if acceptedProtocolConn != nil {
				_ = acceptedProtocolConn.Close()
			}
		}

		neighbor.Events.ProtocolConnectionEstablished.Trigger(neighbor.InitiatedProtocol)
	}))

	return neighbor.InitiatedProtocol, true, nil
}

func (neighbor *Neighbor) Marshal() []byte {
	return nil
}

func (neighbor *Neighbor) Equals(other *Neighbor) bool {
	return neighbor.Identity.StringIdentifier == other.Identity.StringIdentifier &&
		neighbor.Port == other.Port && neighbor.Address.String() == other.Address.String()
}

func AddNeighbor(newNeighbor *Neighbor) {
	neighborLock.Lock()
	defer neighborLock.Unlock()

	if neighbor, exists := neighbors[newNeighbor.Identity.StringIdentifier]; !exists {
		neighbors[newNeighbor.Identity.StringIdentifier] = newNeighbor

		Events.AddNeighbor.Trigger(newNeighbor)
	} else {
		if !neighbor.Equals(newNeighbor) {
			neighbor.Identity = newNeighbor.Identity
			neighbor.Port = newNeighbor.Port
			neighbor.Address = newNeighbor.Address

			Events.UpdateNeighbor.Trigger(neighbor)
		}
	}
}

func RemoveNeighbor(identifier string) {
	if _, exists := neighbors[identifier]; exists {
		neighborLock.Lock()
		defer neighborLock.Unlock()

		if neighbor, exists := neighbors[identifier]; exists {
			delete(neighbors, identifier)

			Events.RemoveNeighbor.Trigger(neighbor)
		}
	}
}

func GetNeighbor(identifier string) (*Neighbor, bool) {
	neighborLock.RLock()
	defer neighborLock.RUnlock()

	neighbor, exists := neighbors[identifier]

	return neighbor, exists
}

func GetNeighbors() map[string]*Neighbor {
	neighborLock.RLock()
	defer neighborLock.RUnlock()

	result := make(map[string]*Neighbor)
	for id, neighbor := range neighbors {
		result[id] = neighbor
	}

	return result
}

const (
	CONNECTION_MAX_ATTEMPTS = 5
	CONNECTION_BASE_TIMEOUT = 10 * time.Second
)

var neighbors = make(map[string]*Neighbor)

var neighborLock sync.RWMutex
