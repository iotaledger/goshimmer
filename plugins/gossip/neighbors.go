package gossip

import (
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/packages/network"
    "net"
    "sync"
)

type Peer struct {
    Identity identity.Identity
    Address  net.IP
    Port     uint16
    Conn     *network.ManagedConnection
}

func UnmarshalPeer(data []byte) (*Peer, error) {
    return &Peer{}, nil
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
    defer neighborLock.Lock()

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
    neighborLock.Lock()
    defer neighborLock.Lock()

    if neighbor, exists := neighbors[identifier]; exists {
        delete(neighbors, identifier)

        Events.RemoveNeighbor.Trigger(neighbor)
    }
}

func GetNeighbor(neighbor *Peer) (*Peer, bool) {
    neighborLock.RLock()
    defer neighborLock.RUnlock()

    neighbor, exists := neighbors[neighbor.Identity.StringIdentifier]

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
    MARSHALLED_NEIGHBOR_TOTAL_SIZE = 1
)

var neighbors = make(map[string]*Peer)
var neighborLock sync.RWMutex
