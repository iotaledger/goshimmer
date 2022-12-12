package metricscollector

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/hive.go/core/generics/event"
)

const (
	autopeeringNamespace = "autopeering"

	neighborDropCount = "neighbor_drop_count"
	connectionsCount  = "neighbor_connections_count"
	// add utility functions about distance directly to autopeering package, or utils for autopeering
	distance                    = "distance"
	neighborConnectionLifetimeS = "neighbor_connection_lifetime_s"
)

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
	collector.WithMetric(collector.NewMetric(neighborConnectionLifetimeS,
		collector.WithType(collector.Counter),
		collector.WithHelp("Time since neighbor connection establishment"),
		collector.WithInitFunc(func() {
			deps.P2Pmgr.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Attach(event.NewClosure(func(event *p2p.NeighborRemovedEvent) {
				neighborConnectionsLifeTime := time.Since(event.Neighbor.ConnectionEstablished())
				deps.Collector.Update(autopeeringNamespace, neighborConnectionLifetimeS, collector.SingleValue(neighborConnectionsLifeTime.Seconds()))
			}))
		}),
	)),
)
