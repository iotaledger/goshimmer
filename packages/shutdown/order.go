package shutdown

const (
	ShutdownPriorityTangle = iota
	ShutdownPriorityRemoteLog
	ShutdownPriorityAnalysis
	ShutdownPriorityMetrics
	ShutdownPriorityAutopeering
	ShutdownPriorityGossip
	ShutdownPriorityWebAPI
	ShutdownPrioritySPA
	ShutdownPriorityGraph
	ShutdownPrioritySpammer
	ShutdownPriorityBadgerGarbageCollection
)
