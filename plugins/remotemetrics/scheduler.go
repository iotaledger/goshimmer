package remotemetrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/app/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/congestioncontrol/icca/scheduler"
)

func obtainSchedulerStats(timestamp time.Time) {
	scheduler := deps.Protocol.Instance().CongestionControl.Scheduler
	queueMap, aManaNormalizedMap := prepQueueMaps(scheduler)

	var myID string
	if deps.Local != nil {
		myID = deps.Local.Identity.ID().String()
	}
	record := remotemetrics.SchedulerMetrics{
		Type:                         "schedulerSample",
		NodeID:                       myID,
		Synced:                       deps.Protocol.Instance().IsSynced(),
		MetricsLevel:                 Parameters.MetricsLevel,
		BufferSize:                   uint32(scheduler.BufferSize()),
		BufferLength:                 uint32(scheduler.TotalBlocksCount()),
		ReadyBlocksInBuffer:          uint32(scheduler.ReadyBlocksCount()),
		QueueLengthPerNode:           queueMap,
		AManaNormalizedLengthPerNode: aManaNormalizedMap,
		Timestamp:                    timestamp,
	}

	_ = deps.RemoteLogger.Send(record)
}

func prepQueueMaps(s *scheduler.Scheduler) (queueMap map[string]uint32, aManaNormalizedMap map[string]float64) {
	queueSizes := s.IssuerQueueSizes()
	queueMap = make(map[string]uint32, len(queueSizes))
	aManaNormalizedMap = make(map[string]float64, len(queueSizes))

	// TODO: implement when mana is refactored
	// for id, size := range queueSizes {
	//	nodeID := id.String()
	//	aMana := s.GetManaFromCache(id)
	//
	//	queueMap[nodeID] = uint32(size)
	//	aManaNormalizedMap[nodeID] = float64(size) / float64(aMana)
	// }
	return
}
