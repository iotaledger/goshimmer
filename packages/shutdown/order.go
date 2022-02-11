package shutdown

const (
	// PriorityDatabase defines the shutdown priority for the database.
	PriorityDatabase = iota
	// PriorityPeerDatabase defines the shutdown priority for the peer database.
	PriorityPeerDatabase
	// PriorityMana defines the shutdown priority for the mana plugin.
	PriorityMana
	// PriorityTangle defines the shutdown priority for the tangle.
	PriorityTangle
	// PriorityDRNG defines the shutdown priority for dRNG.
	PriorityDRNG
	// PriorityFaucet defines the shutdown priority for the faucet.
	PriorityFaucet
	// PriorityRemoteLog defines the shutdown priority for remote log.
	PriorityRemoteLog
	// PriorityAnalysis defines the shutdown priority for analysis server.
	PriorityAnalysis
	// PriorityPrometheus defines the shutdown priority for prometheus.
	PriorityPrometheus
	// PriorityMetrics defines the shutdown priority for metrics server.
	PriorityMetrics
	// PriorityGossip defines the shutdown priority for gossip.
	PriorityGossip
	// PriorityAutopeering defines the shutdown priority for autopeering.
	PriorityAutopeering
	// PriorityManualpeering defines the shutdown priority for manualpeering.
	PriorityManualpeering
	// PriorityFirewall defines the shutdown priority for firewall.
	PriorityFirewall
	// PriorityWebAPI defines the shutdown priority for webapi.
	PriorityWebAPI
	// PriorityDashboard defines the shutdown priority for dashboard.
	PriorityDashboard
	// PriorityBroadcast defines the shutdown priority for the broadcast plugin.
	PriorityBroadcast
	// PrioritySynchronization defines the shutdown priority for synchronization.
	PrioritySynchronization
	// PriorityManaRefresher defines the shutdown priority for the manarefresher plugin.
	PriorityManaRefresher
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
