package shutdown

const (
	PriorityDatabase = iota
	PriorityFPC
	PriorityTangle
	PriorityMissingMessagesMonitoring
	PriorityFaucet
	PriorityRemoteLog
	PriorityAnalysis
	PriorityPrometheus
	PriorityMetrics
	PriorityAutopeering
	PriorityGossip
	PriorityWebAPI
	PriorityDashboard
	PrioritySynchronization
	PriorityBootstrap
	PrioritySpammer
)
