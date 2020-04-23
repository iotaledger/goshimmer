package recordedevents

import (
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/heartbeat"
	"github.com/iotaledger/goshimmer/plugins/analysis/webinterface/types"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
)

// Maps nodeId to the latest arrival of a heartbeat
var nodes = make(map[string]time.Time)

// Maps nodeId to outgoing connections + latest arrival of heartbeat
var links = make(map[string]map[string]time.Time)

var lock sync.Mutex

// Configure configures the plugin.
func Configure(plugin *node.Plugin) {
	server.Events.Heartbeat.Attach(events.NewClosure(func(packet heartbeat.Packet) {
		var out strings.Builder
		for _, value := range packet.OutboundIDs {
			out.WriteString(hex.EncodeToString(value))
		}
		var in strings.Builder
		for _, value := range packet.InboundIDs {
			in.WriteString(hex.EncodeToString(value))
		}
		plugin.Node.Logger.Debugw(
			"Heartbeat",
			"nodeId", hex.EncodeToString(packet.OwnID),
			"outboundIds", out.String(),
			"inboundIds", in.String(),
		)
		lock.Lock()
		defer lock.Unlock()

		nodeIDString := hex.EncodeToString(packet.OwnID)
		timestamp := time.Now()

		// When node is new, add to graph
		if _, isAlready := nodes[nodeIDString]; !isAlready {
			server.Events.AddNode.Trigger(nodeIDString)
		}
		// Save it + update timestamp
		nodes[nodeIDString] = timestamp

		// Outgoing neighbor links update
		for _, outgoingNeighbor := range packet.OutboundIDs {
			outgoingNeighborString := hex.EncodeToString(outgoingNeighbor)
			// Do we already know about this neighbor?
			// If no, add it and set it online
			if _, isAlready := nodes[outgoingNeighborString]; !isAlready {
				// First time we see this particular node
				server.Events.AddNode.Trigger(outgoingNeighborString)
			}
			// We have indirectly heard about the neighbor.
			nodes[outgoingNeighborString] = timestamp

			// Do we have any links already with src=nodeIdString?
			if _, isAlready := links[nodeIDString]; !isAlready {
				// Nope, so we have to allocate an empty map to be nested in links for nodeIdString
				links[nodeIDString] = make(map[string]time.Time)
			}

			// Update graph when connection hasn't been seen before
			if _, isAlready := links[nodeIDString][outgoingNeighborString]; !isAlready {
				server.Events.ConnectNodes.Trigger(nodeIDString, outgoingNeighborString)
			}
			// Update links
			links[nodeIDString][outgoingNeighborString] = timestamp
		}

		// Incoming neighbor links update
		for _, incomingNeighbor := range packet.InboundIDs {
			incomingNeighborString := hex.EncodeToString(incomingNeighbor)
			// Do we already know about this neighbor?
			// If no, add it and set it online
			if _, isAlready := nodes[incomingNeighborString]; !isAlready {
				// First time we see this particular node
				server.Events.AddNode.Trigger(incomingNeighborString)
			}
			// We have indirectly heard about the neighbor.
			nodes[incomingNeighborString] = timestamp

			// Do we have any links already with src=incomingNeighborString?
			if _, isAlready := links[incomingNeighborString]; !isAlready {
				// Nope, so we have to allocate an empty map to be nested in links for incomingNeighborString
				links[incomingNeighborString] = make(map[string]time.Time)
			}

			// Update graph when connection hasn't been seen before
			if _, isAlready := links[incomingNeighborString][nodeIDString]; !isAlready {
				server.Events.ConnectNodes.Trigger(incomingNeighborString, nodeIDString)
			}
			// Update links map
			links[incomingNeighborString][nodeIDString] = timestamp
		}
	}))
}

// Run runs the plugin.
func Run() {
	daemon.BackgroundWorker("Analysis Server Record Manager", func(shutdownSignal <-chan struct{}) {
		ticker := time.NewTicker(CleanUpPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-shutdownSignal:
				return

			case <-ticker.C:
				cleanUp(CleanUpPeriod)
			}
		}
	}, shutdown.PriorityAnalysis)
}

// Remove nodes and links we haven't seen for at least 3 times the heartbeat interval
func cleanUp(interval time.Duration) {
	lock.Lock()
	defer lock.Unlock()
	now := time.Now()

	// Go through the list of connections. Remove connections that are older than interval time.
	for srcNode, targetMap := range links {
		for trgNode, lastSeen := range targetMap {
			if now.Sub(lastSeen) > interval {
				delete(targetMap, trgNode)
				server.Events.DisconnectNodes.Trigger(srcNode, trgNode)
			}
		}
		// Delete src node from links if it doesn't have any connections
		if len(targetMap) == 0 {
			delete(links, srcNode)
		}
	}

	// Go through the list of nodes. Remove nodes that haven't been seen for interval time
	for node, lastSeen := range nodes {
		if now.Sub(lastSeen) > interval {
			delete(nodes, node)
			server.Events.RemoveNode.Trigger(node)
		}
	}
}

func getEventsToReplay() (map[string]time.Time, map[string]map[string]time.Time) {
	lock.Lock()
	defer lock.Unlock()

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

// Replay runs the handlers on the events to replay.
func Replay(handlers *types.EventHandlers) {
	copiedNodes, copiedLinks := getEventsToReplay()

	// When a node is present in the list, it means we heard about it directly
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
