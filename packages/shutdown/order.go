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
	PriorityGraph
	PrioritySpammer
	PriorityBadgerGarbageCollection
)
