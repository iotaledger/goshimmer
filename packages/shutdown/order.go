package shutdown

const (
	// PriorityDatabase defines the shutdown priority for the database.
	PriorityDatabase = iota
	// PriorityMana defines the shutdown priority for the mana plugin.
	PriorityMana
	// PriorityTangle defines the shutdown priority for the tangle.
	PriorityTangle
	// PriorityFPC defines the shutdown priority for the FPC.
	PriorityFPC
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
	// PriorityAutopeering defines the shutdown priority for autopeering.
	PriorityAutopeering
	// PriorityGossip defines the shutdown priority for gossip.
	PriorityGossip
	// PriorityWebAPI defines the shutdown priority for webapi.
	PriorityWebAPI
	// PriorityDashboard defines the shutdown priority for dashboard.
	PriorityDashboard
	// PrioritySynchronization defines the shutdown priority for synchronization.
	PrioritySynchronization
	// PrioritySpammer defines the shutdown priority for spammer.
	PrioritySpammer
	// PriorityBootstrap defines the shutdown priority for bootstrap.
	PriorityBootstrap
)
