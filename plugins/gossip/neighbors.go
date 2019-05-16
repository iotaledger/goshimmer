package gossip

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/errors"
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/packages/network"
    "github.com/iotaledger/goshimmer/packages/node"
    "math"
    "net"
    "strconv"
    "sync"
    "time"
)

func configureNeighbors(plugin *node.Plugin) {
    Events.AddNeighbor.Attach(events.NewClosure(func(neighbor *Peer) {
        plugin.LogSuccess("new neighbor added " + neighbor.Identity.StringIdentifier + "@" + neighbor.Address.String() + ":" + strconv.Itoa(int(neighbor.Port)))
    }))

    Events.UpdateNeighbor.Attach(events.NewClosure(func(neighbor *Peer) {
        plugin.LogSuccess("existing neighbor updated " + neighbor.Identity.StringIdentifier + "@" + neighbor.Address.String() + ":" + strconv.Itoa(int(neighbor.Port)))
    }))

    Events.RemoveNeighbor.Attach(events.NewClosure(func(neighbor *Peer) {
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

    Events.AddNeighbor.Attach(events.NewClosure(func(neighbor *Peer) {
        manageConnection(plugin, neighbor)
    }))

    plugin.LogSuccess("Starting Neighbor Connection Manager ... done")
}

func manageConnection(plugin *node.Plugin, neighbor *Peer) {
    daemon.BackgroundWorker(func() {
        failedConnectionAttempts := 0

        for _, exists := GetNeighbor(neighbor.Identity.StringIdentifier); exists && failedConnectionAttempts < CONNECTION_MAX_ATTEMPTS; {
            conn, dialed, err := neighbor.Connect()
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

            disconnectChan := make(chan int, 1)
            conn.Events.Close.Attach(events.NewClosure(func() {
                close(disconnectChan)
            }))

            if dialed {
                go newProtocol(conn).Init()
            }

            // wait for shutdown or
            select {
            case <-daemon.ShutdownSignal:
                return

            case <-disconnectChan:
                break
            }
        }

        RemoveNeighbor(neighbor.Identity.StringIdentifier)
    })
}

type Peer struct {
    Identity           *identity.Identity
    Address            net.IP
    Port               uint16
    InitiatedConn      *network.ManagedConnection
    AcceptedConn       *network.ManagedConnection
    initiatedConnMutex sync.RWMutex
    acceptedConnMutex  sync.RWMutex
}

func UnmarshalPeer(data []byte) (*Peer, error) {
    return &Peer{}, nil
}

func (peer *Peer) Connect() (*network.ManagedConnection, bool, errors.IdentifiableError) {
    peer.initiatedConnMutex.Lock()
    defer peer.initiatedConnMutex.Unlock()

    // return existing connections first
    if peer.InitiatedConn != nil {
        return peer.InitiatedConn, false, nil
    }

    // if we already have an accepted connection -> use it instead
    if peer.AcceptedConn != nil {
        peer.acceptedConnMutex.RLock()
        if peer.AcceptedConn != nil {
            defer peer.acceptedConnMutex.RUnlock()

            return peer.AcceptedConn, false, nil
        }
        peer.acceptedConnMutex.RUnlock()
    }

    // otherwise try to dial
    conn, err := net.Dial("tcp", peer.Address.String()+":"+strconv.Itoa(int(peer.Port)))
    if err != nil {
        return nil, false, ErrConnectionFailed.Derive(err, "error when connecting to neighbor "+
            peer.Identity.StringIdentifier+"@"+peer.Address.String()+":"+strconv.Itoa(int(peer.Port)))
    }

    peer.InitiatedConn = network.NewManagedConnection(conn)

    peer.InitiatedConn.Events.Close.Attach(events.NewClosure(func() {
        peer.initiatedConnMutex.Lock()
        defer peer.initiatedConnMutex.Unlock()

        peer.InitiatedConn = nil
    }))

    return peer.InitiatedConn, true, nil
}

func (peer *Peer) Marshal() []byte {
    return nil
}

func (peer *Peer) Equals(other *Peer) bool {
    return peer.Identity.StringIdentifier == peer.Identity.StringIdentifier &&
        peer.Port == other.Port && peer.Address.String() == other.Address.String()
}

func AddNeighbor(newNeighbor *Peer) {
    neighborLock.Lock()
    defer neighborLock.Unlock()

    if neighbor, exists := neighbors[newNeighbor.Identity.StringIdentifier]; !exists {
        neighbors[newNeighbor.Identity.StringIdentifier] = newNeighbor

        Events.AddNeighbor.Trigger(newNeighbor)
    } else {
        if !newNeighbor.Equals(neighbor) {
            neighbor.Identity = neighbor.Identity
            neighbor.Port = neighbor.Port
            neighbor.Address = neighbor.Address

            Events.UpdateNeighbor.Trigger(newNeighbor)
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

func GetNeighbor(identifier string) (*Peer, bool) {
    neighborLock.RLock()
    defer neighborLock.RUnlock()

    neighbor, exists := neighbors[identifier]

    return neighbor, exists
}

func GetNeighbors() map[string]*Peer {
    neighborLock.RLock()
    defer neighborLock.RUnlock()

    result := make(map[string]*Peer)
    for id, neighbor := range neighbors {
        result[id] = neighbor
    }

    return result
}

const (
    CONNECTION_MAX_ATTEMPTS        = 5
    CONNECTION_BASE_TIMEOUT        = 10 * time.Second
)

var neighbors = make(map[string]*Peer)
var neighborLock sync.RWMutex
