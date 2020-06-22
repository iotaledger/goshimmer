package shutdown

const (
	PriorityDatabase = iota
	PriorityFPC
	PriorityTangle
	PriorityMissingMessagesMonitoring
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
	PriorityBadgerGarbageCollection
)
