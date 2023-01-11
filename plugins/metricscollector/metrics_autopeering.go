package metricscollector

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/hive.go/core/autopeering/selection"
	"github.com/iotaledger/hive.go/core/generics/event"
)

const (
	autopeeringNamespace = "autopeering"

	neighborDropCount = "neighbor_drop_total"
	connectionsCount  = "neighbor_connections_total"
	// todo calculate avg, max, min directly in grafana
	distance                      = "distance"
	neighborConnectionLifetimeSec = "autopeering_neighbor_connection_lifetime_seconds_total"
	trafficInboundBytes           = "traffic_inbound_total_bytes"
	trafficOutboundBytes          = "traffic_outbound_total_bytes"
)

// AutopeeringMetrics is the collection of metrics for autopeering component.
var AutopeeringMetrics = collector.NewCollection(autopeeringNamespace,
	collector.WithMetric(collector.NewMetric(neighborDropCount,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of dropped neighbors so far"),
		collector.WithInitFunc(func() {
			deps.P2Pmgr.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Attach(event.NewClosure(func(event *p2p.NeighborRemovedEvent) {
				deps.Collector.Increment(autopeeringNamespace, neighborDropCount)
			}))
		}),
	)),
	collector.WithMetric(collector.NewMetric(connectionsCount,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of established neighbor connections so far"),
		collector.WithInitFunc(func() {
			deps.P2Pmgr.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborAdded.Attach(event.NewClosure(func(event *p2p.NeighborAddedEvent) {
				deps.Collector.Increment(autopeeringNamespace, connectionsCount)
			}))
		}),
	)),
	collector.WithMetric(collector.NewMetric(neighborConnectionLifetimeSec,
		collector.WithType(collector.Counter),
		collector.WithHelp("Time since neighbor connection establishment"),
		collector.WithInitFunc(func() {
			deps.P2Pmgr.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Attach(event.NewClosure(func(event *p2p.NeighborRemovedEvent) {
				neighborConnectionsLifeTime := time.Since(event.Neighbor.ConnectionEstablished())
				deps.Collector.Update(autopeeringNamespace, neighborConnectionLifetimeSec, collector.SingleValue(neighborConnectionsLifeTime.Seconds()))
			}))
		}),
	)),
	collector.WithMetric(collector.NewMetric(distance,
		collector.WithType(collector.Gauge),
		collector.WithHelp("A relative distance between the node and the neighbor"),
		collector.WithInitFunc(func() {
			var onAutopeeringSelection = event.NewClosure(
				func(event *selection.PeeringEvent) {
					deps.Collector.Update(autopeeringNamespace, distance, collector.SingleValue(float64(event.Distance)))
				})
			if deps.Selection != nil {
				deps.Selection.Events().IncomingPeering.Hook(onAutopeeringSelection)
				deps.Selection.Events().OutgoingPeering.Hook(onAutopeeringSelection)
			}
		}),
	)),
	collector.WithMetric(collector.NewMetric(trafficInboundBytes,
		collector.WithType(collector.Counter),
		collector.WithHelp("Inbound network autopeering traffic in bytes"),
		collector.WithCollectFunc(func() map[string]float64 {
			return collector.SingleValue(deps.AutoPeeringConnMetric.RXBytes())
		}),
	)),
	collector.WithMetric(collector.NewMetric(trafficOutboundBytes,
		collector.WithType(collector.Counter),
		collector.WithHelp("Outbound network autopeering traffic in bytes"),
		collector.WithCollectFunc(func() map[string]float64 {
			return collector.SingleValue(deps.AutoPeeringConnMetric.TXBytes())
		}),
	)),
)
