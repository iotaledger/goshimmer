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
	// PriorityGossip defines the shutdown priority for gossip.
	PriorityGossip
	// PriorityAutopeering defines the shutdown priority for autopeering.
	PriorityAutopeering
	// PriorityManualpeering defines the shutdown priority for manualpeering.
	PriorityManualpeering
	// PriorityWebAPI defines the shutdown priority for webapi.
	PriorityWebAPI
	// PriorityDashboard defines the shutdown priority for dashboard.
	PriorityDashboard
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
)
