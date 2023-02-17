package shutdown

const (
	// PriorityDatabase defines the shutdown priority for the database.
	PriorityDatabase = iota
	// PriorityPeerDatabase defines the shutdown priority for the peer database.
	PriorityPeerDatabase
	// PriorityMana defines the shutdown priority for the mana plugin.
	PriorityMana
	// PriorityNotarization defines the shutdown priority for the notarization.
	PriorityNotarization
	// PriorityTangle defines the shutdown priority for the tangle.
	PriorityTangle
	// PriorityFaucet defines the shutdown priority for the faucet.
	PriorityFaucet
	// PriorityRemoteLog defines the shutdown priority for remote log.
	PriorityRemoteLog
	// PriorityProfiling defines the shutdown priority for profiling.
	PriorityProfiling
	// PriorityPrometheus defines the shutdown priority for prometheus.
	PriorityPrometheus
	// PriorityMetrics defines the shutdown priority for metrics server.
	PriorityMetrics
	// PriorityWarpsync defines the shutdown priority for warpsync.
	PriorityWarpsync
	// PriorityGossip defines the shutdown priority for gossip.
	PriorityGossip
	// PriorityP2P defines the shutdown priority for p2p.
	PriorityP2P
	// PriorityAutopeering defines the shutdown priority for autopeering.
	PriorityAutopeering
	// PriorityManualpeering defines the shutdown priority for manualpeering.
	PriorityManualpeering
	// PriorityWebAPI defines the shutdown priority for webapi.
	PriorityWebAPI
	// PriorityDashboard defines the shutdown priority for dashboard.
	PriorityDashboard
	// PriorityBroadcast defines the shutdown priority for the broadcast plugin.
	PriorityBroadcast
	// PrioritySynchronization defines the shutdown priority for synchronization.
	PrioritySynchronization
	// PriorityActivity defines the shutdown priority for the activity plugin.
	PriorityActivity
	// PrioritySpammer defines the shutdown priority for spammer.
	PrioritySpammer
	// PriorityBootstrap defines the shutdown priority for bootstrap.
	PriorityBootstrap
	// PriorityTXStream defines the shutdown priority for realtime.
	PriorityTXStream
	// PriorityHealthz defines the shutdown priority of the healthz endpoint. It should always be last.
	PriorityHealthz
)
