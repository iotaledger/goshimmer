package dashboard

import (
	"strings"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/graph"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	analysisserver "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
)

// the period in which we scan and delete old data.
const cleanUpPeriod = 15 * time.Second

var (
	// maps nodeId to the latest arrival of a heartbeat.
	nodes = make(map[string]time.Time)
	// maps nodeId to outgoing connections + latest arrival of heartbeat.
	links = make(map[string]map[string]time.Time)
	lock  sync.RWMutex
)

// NeighborMetric contains the number of inbound/outbound neighbors.
type NeighborMetric struct {
	Inbound  uint
	Outbound uint
}

// NumOfNeighbors returns a map of nodeIDs to their neighbor count.
func NumOfNeighbors() map[string]*NeighborMetric {
	lock.RLock()
	defer lock.RUnlock()
	result := make(map[string]*NeighborMetric)
	for nodeID := range nodes {
		// number of outgoing neighbors
		if _, exist := result[nodeID]; !exist {
			result[nodeID] = &NeighborMetric{Outbound: uint(len(links[nodeID]))}
		} else {
			result[nodeID].Outbound = uint(len(links[nodeID]))
		}

		// fill in incoming neighbors
		for outNeighborID := range links[nodeID] {
			if _, exist := result[outNeighborID]; !exist {
				result[outNeighborID] = &NeighborMetric{Inbound: 1}
			} else {
				result[outNeighborID].Inbound++
			}
		}
	}
	return result
}

// NetworkGraph returns the autopeering network graph.
func NetworkGraph() *graph.Graph {
	lock.RLock()
	defer lock.RUnlock()
	var nodeIDs []string
	for id := range nodes {
		nodeIDs = append(nodeIDs, id)
	}
	g := graph.New(nodeIDs)

	for src, trgMap := range links {
		for dst := range trgMap {
			g.AddEdge(src, dst)
		}
	}
	return g
}

// configures the event recording by attaching to the analysis server's events.
func configureEventsRecording() {
	analysisserver.Events.Heartbeat.Attach(events.NewClosure(func(hb *packet.Heartbeat) {
		var out strings.Builder
		for _, value := range hb.OutboundIDs {
			out.WriteString(shortNodeIDString(value))
		}
		var in strings.Builder
		for _, value := range hb.InboundIDs {
			in.WriteString(shortNodeIDString(value))
		}
		log.Debugw(
			"Heartbeat",
			"nodeId", shortNodeIDString(hb.OwnID),
			"outboundIds", out.String(),
			"inboundIds", in.String(),
		)
		lock.Lock()
		defer lock.Unlock()

		nodeIDString := shortNodeIDString(hb.OwnID)
		timestamp := time.Now()

		// when node is new, add to graph
		if _, isAlready := nodes[nodeIDString]; !isAlready {
			analysisserver.Events.AddNode.Trigger(nodeIDString)
		}
		// save it + update timestamp
		nodes[nodeIDString] = timestamp

		// outgoing neighbor links update
		for _, outgoingNeighbor := range hb.OutboundIDs {
			outgoingNeighborString := shortNodeIDString(outgoingNeighbor)
			// do we already know about this neighbor?
			// if no, add it and set it online
			if _, isAlready := nodes[outgoingNeighborString]; !isAlready {
				// first time we see this particular node
				analysisserver.Events.AddNode.Trigger(outgoingNeighborString)
			}
			// we have indirectly heard about the neighbor.
			nodes[outgoingNeighborString] = timestamp

			// do we have any links already with src=nodeIdString?
			if _, isAlready := links[nodeIDString]; !isAlready {
				// nope, so we have to allocate an empty map to be nested in links for nodeIdString
				links[nodeIDString] = make(map[string]time.Time)
			}

			// update graph when connection hasn't been seen before
			if _, isAlready := links[nodeIDString][outgoingNeighborString]; !isAlready {
				analysisserver.Events.ConnectNodes.Trigger(nodeIDString, outgoingNeighborString)
			}
			// update links
			links[nodeIDString][outgoingNeighborString] = timestamp
		}

		// incoming neighbor links update
		for _, incomingNeighbor := range hb.InboundIDs {
			incomingNeighborString := shortNodeIDString(incomingNeighbor)
			// do we already know about this neighbor?
			// if no, add it and set it online
			if _, isAlready := nodes[incomingNeighborString]; !isAlready {
				// First time we see this particular node
				analysisserver.Events.AddNode.Trigger(incomingNeighborString)
			}
			// we have indirectly heard about the neighbor.
			nodes[incomingNeighborString] = timestamp

			// do we have any links already with src=incomingNeighborString?
			if _, isAlready := links[incomingNeighborString]; !isAlready {
				// nope, so we have to allocate an empty map to be nested in links for incomingNeighborString
				links[incomingNeighborString] = make(map[string]time.Time)
			}

			// update graph when connection hasn't been seen before
			if _, isAlready := links[incomingNeighborString][nodeIDString]; !isAlready {
				analysisserver.Events.ConnectNodes.Trigger(incomingNeighborString, nodeIDString)
			}
			// update links map
			links[incomingNeighborString][nodeIDString] = timestamp
		}
	}))
}

// starts record manager that initiates a record cleanup periodically
func runEventsRecordManager() {
	if err := daemon.BackgroundWorker("Dashboard Analysis Server Record Manager", func(shutdownSignal <-chan struct{}) {
		ticker := time.NewTicker(cleanUpPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-shutdownSignal:
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
	lock.Lock()
	defer lock.Unlock()
	now := time.Now()

	// go through the list of connections. Remove connections that are older than interval time.
	for srcNode, targetMap := range links {
		for trgNode, lastSeen := range targetMap {
			if now.Sub(lastSeen) > interval {
				delete(targetMap, trgNode)
				analysisserver.Events.DisconnectNodes.Trigger(srcNode, trgNode)
			}
		}
		// delete src node from links if it doesn't have any connections
		if len(targetMap) == 0 {
			delete(links, srcNode)
		}
	}

	// go through the list of nodes. Remove nodes that haven't been seen for interval time
	for node, lastSeen := range nodes {
		if now.Sub(lastSeen) > interval {
			delete(nodes, node)
			analysisserver.Events.RemoveNode.Trigger(node)
		}
	}
}

func getEventsToReplay() (map[string]time.Time, map[string]map[string]time.Time) {
	lock.RLock()
	defer lock.RUnlock()

	copiedNodes := make(map[string]time.Time, len(nodes))
	for nodeID, lastHeartbeat := range nodes {
		copiedNodes[nodeID] = lastHeartbeat
	}

	copiedLinks := make(map[string]map[string]time.Time, len(links))
	for sourceID, targetMap := range links {
		copiedLinks[sourceID] = make(map[string]time.Time, len(targetMap))
		for targetID, lastHeartbeat := range targetMap {
			copiedLinks[sourceID][targetID] = lastHeartbeat
		}
	}

	return copiedNodes, copiedLinks
}

// replays recorded events on the given event handler.
func replayAutopeeringEvents(handlers *EventHandlers) {
	copiedNodes, copiedLinks := getEventsToReplay()

	// when a node is present in the list, it means we heard about it directly
	// or indirectly, but within CLEAN_UP_PERIOD, therefore it is online
	for nodeID := range copiedNodes {
		handlers.AddNode(nodeID)
	}

	for sourceID, targetMap := range copiedLinks {
		for targetID := range targetMap {
			handlers.ConnectNodes(sourceID, targetID)
		}
	}
}

// EventHandlers holds the handler for each event of the record manager.
type EventHandlers struct {
	// Addnode defines the handler called when adding a new node.
	AddNode func(nodeId string)
	// RemoveNode defines the handler called when adding removing a node.
	RemoveNode func(nodeId string)
	// ConnectNodes defines the handler called when connecting two nodes.
	ConnectNodes func(sourceId string, targetId string)
	// DisconnectNodes defines the handler called when connecting two nodes.
	DisconnectNodes func(sourceId string, targetId string)
}

// EventHandlersConsumer defines the consumer function of an *EventHandlers.
type EventHandlersConsumer = func(handler *EventHandlers)

func shortNodeIDString(b []byte) string {
	var id identity.ID
	copy(id[:], b)
	return id.String()
}
