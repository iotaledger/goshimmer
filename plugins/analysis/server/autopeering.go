package server

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/graph"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
)

// NetworkMap contains information about the peer connections on a specific network.
type NetworkMap struct {
	version string
	// maps nodeId to the latest arrival of a heartbeat.
	nodes map[string]time.Time
	// maps nodeId to outgoing connections + latest arrival of heartbeat.
	links map[string]map[string]time.Time
	lock  sync.RWMutex
}

// NeighborMetric contains the number of inbound/outbound neighbors.
type NeighborMetric struct {
	Inbound  uint
	Outbound uint
}

// Networks maps all available versions to network map
var Networks = make(map[string]*NetworkMap)

func updateAutopeeringMap(p *packet.Heartbeat) {
	networkID := string(p.NetworkID)
	var out strings.Builder
	for _, value := range p.OutboundIDs {
		out.WriteString(ShortNodeIDString(value))
	}
	var in strings.Builder
	for _, value := range p.InboundIDs {
		in.WriteString(ShortNodeIDString(value))
	}
	log.Debugw(
		"Heartbeat",
		"networkID", networkID,
		"nodeId", ShortNodeIDString(p.OwnID),
		"outboundIds", out.String(),
		"inboundIds", in.String(),
	)
	if _, ok := Networks[networkID]; !ok {
		// first time we see this network
		Networks[networkID] = NewNetworkMap(networkID)
	}
	nm := Networks[networkID]
	nm.update(p)
}

// NewNetworkMap creates a new network map with a given network version
func NewNetworkMap(networkVersion string) *NetworkMap {
	nm := &NetworkMap{
		version: networkVersion,
		nodes:   make(map[string]time.Time),
		links:   make(map[string]map[string]time.Time),
	}
	return nm
}

func (nm *NetworkMap) update(hb *packet.Heartbeat) {
	nm.lock.Lock()
	defer nm.lock.Unlock()

	nodeIDString := ShortNodeIDString(hb.OwnID)
	timestamp := time.Now()

	// when node is new, add to graph
	if _, isAlready := nm.nodes[nodeIDString]; !isAlready {
		Events.AddNode.Trigger(&AddNodeEvent{NetworkVersion: nm.version, NodeID: nodeIDString})
	}
	// save it + update timestamp
	nm.nodes[nodeIDString] = timestamp

	// outgoing neighbor links update
	for _, outgoingNeighbor := range hb.OutboundIDs {
		outgoingNeighborString := ShortNodeIDString(outgoingNeighbor)
		// do we already know about this neighbor?
		// if no, add it and set it online
		if _, isAlready := nm.nodes[outgoingNeighborString]; !isAlready {
			// first time we see this particular node
			Events.AddNode.Trigger(&AddNodeEvent{NetworkVersion: nm.version, NodeID: outgoingNeighborString})
		}
		// we have indirectly heard about the neighbor.
		nm.nodes[outgoingNeighborString] = timestamp

		// do we have any links already with src=nodeIdString?
		if _, isAlready := nm.links[nodeIDString]; !isAlready {
			// nope, so we have to allocate an empty map to be nested in links for nodeIdString
			nm.links[nodeIDString] = make(map[string]time.Time)
		}

		// update graph when connection hasn't been seen before
		if _, isAlready := nm.links[nodeIDString][outgoingNeighborString]; !isAlready {
			Events.ConnectNodes.Trigger(&ConnectNodesEvent{NetworkVersion: nm.version, SourceID: nodeIDString, TargetID: outgoingNeighborString})
		}
		// update links
		nm.links[nodeIDString][outgoingNeighborString] = timestamp
	}

	// incoming neighbor links update
	for _, incomingNeighbor := range hb.InboundIDs {
		incomingNeighborString := ShortNodeIDString(incomingNeighbor)
		// do we already know about this neighbor?
		// if no, add it and set it online
		if _, isAlready := nm.nodes[incomingNeighborString]; !isAlready {
			// First time we see this particular node
			Events.AddNode.Trigger(&AddNodeEvent{NetworkVersion: nm.version, NodeID: incomingNeighborString})
		}
		// we have indirectly heard about the neighbor.
		nm.nodes[incomingNeighborString] = timestamp

		// do we have any links already with src=incomingNeighborString?
		if _, isAlready := nm.links[incomingNeighborString]; !isAlready {
			// nope, so we have to allocate an empty map to be nested in links for incomingNeighborString
			nm.links[incomingNeighborString] = make(map[string]time.Time)
		}

		// update graph when connection hasn't been seen before
		if _, isAlready := nm.links[incomingNeighborString][nodeIDString]; !isAlready {
			Events.ConnectNodes.Trigger(&ConnectNodesEvent{NetworkVersion: nm.version, SourceID: incomingNeighborString, TargetID: nodeIDString})
		}
		// update links map
		nm.links[incomingNeighborString][nodeIDString] = timestamp
	}
}

// starts record manager that initiates a record cleanup periodically
func runEventsRecordManager() {
	if err := daemon.BackgroundWorker("Analysis Server Autopeering Record Manager", func(ctx context.Context) {
		ticker := time.NewTicker(cleanUpPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanUp(cleanUpPeriod)
			}
		}
	}, shutdown.PriorityAnalysis); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// removes nodes and links we haven't seen for at least 3 times the heartbeat interval.
func cleanUp(interval time.Duration) {
	for _, networkMap := range Networks {
		networkMap.lock.Lock()
		now := time.Now()

		// go through the list of connections. Remove connections that are older than interval time.
		for srcNode, targetMap := range networkMap.links {
			for trgNode, lastSeen := range targetMap {
				if now.Sub(lastSeen) > interval {
					delete(targetMap, trgNode)
					Events.DisconnectNodes.Trigger(&DisconnectNodesEvent{NetworkVersion: networkMap.version, SourceID: srcNode, TargetID: trgNode})
				}
			}
			// delete src node from links if it doesn't have any connections
			if len(targetMap) == 0 {
				delete(networkMap.links, srcNode)
			}
		}

		// go through the list of nodes. Remove nodes that haven't been seen for interval time
		for node, lastSeen := range networkMap.nodes {
			if now.Sub(lastSeen) > interval {
				delete(networkMap.nodes, node)
				Events.RemoveNode.Trigger(&RemoveNodeEvent{NetworkVersion: networkMap.version, NodeID: node})
			}
		}
		networkMap.lock.Unlock()
	}
}

func (nm *NetworkMap) getEventsToReplay() (map[string]time.Time, map[string]map[string]time.Time) {
	nm.lock.RLock()
	defer nm.lock.RUnlock()

	copiedNodes := make(map[string]time.Time, len(nm.nodes))
	for nodeID, lastHeartbeat := range nm.nodes {
		copiedNodes[nodeID] = lastHeartbeat
	}

	copiedLinks := make(map[string]map[string]time.Time, len(nm.links))
	for sourceID, targetMap := range nm.links {
		copiedLinks[sourceID] = make(map[string]time.Time, len(targetMap))
		for targetID, lastHeartbeat := range targetMap {
			copiedLinks[sourceID][targetID] = lastHeartbeat
		}
	}

	return copiedNodes, copiedLinks
}

// ReplayAutopeeringEvents replays recorded events on the given event handler.
func ReplayAutopeeringEvents(handlers *EventHandlers) {
	for _, network := range Networks {
		copiedNodes, copiedLinks := network.getEventsToReplay()

		// when a node is present in the list, it means we heard about it directly
		// or indirectly, but within CLEAN_UP_PERIOD, therefore it is online
		for nodeID := range copiedNodes {
			handlers.AddNode(&AddNodeEvent{NetworkVersion: network.version, NodeID: nodeID})
		}

		for sourceID, targetMap := range copiedLinks {
			for targetID := range targetMap {
				handlers.ConnectNodes(&ConnectNodesEvent{NetworkVersion: network.version, SourceID: sourceID, TargetID: targetID})
			}
		}
	}
}

// EventHandlers holds the handler for each event of the record manager.
type EventHandlers struct {
	// Addnode defines the handler called when adding a new node.
	AddNode func(event *AddNodeEvent)
	// RemoveNode defines the handler called when adding removing a node.
	RemoveNode func(event *RemoveNodeEvent)
	// ConnectNodes defines the handler called when connecting two nodes.
	ConnectNodes func(event *ConnectNodesEvent)
	// DisconnectNodes defines the handler called when connecting two nodes.
	DisconnectNodes func(event *DisconnectNodesEvent)
}

// EventHandlersConsumer defines the consumer function of an *EventHandlers.
type EventHandlersConsumer = func(handler *EventHandlers)

//// Methods for metrics calculation plugin ////

// NumOfNeighbors returns a map of nodeIDs to their neighbor count.
func (nm *NetworkMap) NumOfNeighbors() map[string]*NeighborMetric {
	nm.lock.RLock()
	defer nm.lock.RUnlock()
	result := make(map[string]*NeighborMetric)
	for nodeID := range nm.nodes {
		// number of outgoing neighbors
		if _, exist := result[nodeID]; !exist {
			result[nodeID] = &NeighborMetric{Outbound: uint(len(nm.links[nodeID]))}
		} else {
			result[nodeID].Outbound = uint(len(nm.links[nodeID]))
		}

		// fill in incoming neighbors
		for outNeighborID := range nm.links[nodeID] {
			if _, exist := result[outNeighborID]; !exist {
				result[outNeighborID] = &NeighborMetric{Inbound: 1}
			} else {
				result[outNeighborID].Inbound++
			}
		}
	}
	return result
}

// NetworkGraph returns the autopeering network graph for the current network.
func (nm *NetworkMap) NetworkGraph() *graph.Graph {
	nm.lock.RLock()
	defer nm.lock.RUnlock()
	var nodeIDs []string
	for id := range nm.nodes {
		nodeIDs = append(nodeIDs, id)
	}
	g := graph.New(nodeIDs)

	for src, trgMap := range nm.links {
		for dst := range trgMap {
			g.AddEdge(src, dst)
		}
	}
	return g
}

// ShortNodeIDString returns the short nodeID as a string.
func ShortNodeIDString(b []byte) string {
	var id identity.ID
	copy(id[:], b)
	return id.String()
}
