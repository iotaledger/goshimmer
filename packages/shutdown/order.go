package shutdown

const (
	PriorityDatabase = iota
	PriorityFPC
	PriorityTangle
	PriorityRemoteLog
	PriorityAnalysis
	PriorityMetrics
	PriorityAutopeering
	PriorityGossip
	PriorityWebAPI
	PriorityDashboard
	PrioritySynchronization
	PriorityGraph
	PrioritySpammer
	PriorityBadgerGarbageCollection
)
