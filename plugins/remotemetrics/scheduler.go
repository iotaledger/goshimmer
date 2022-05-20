package remotemetrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

func obtainSchedulerStats(timestamp time.Time) {
	scheduler := deps.Tangle.Scheduler
	queueMap, aManaNormalizedMap := prepQueueMaps(scheduler)

	var myID string
	if deps.Local != nil {
		myID = deps.Local.Identity.ID().String()
	}
	record := remotemetrics.SchedulerMetrics{
		Type:                         "schedulerSample",
		NodeID:                       myID,
		Synced:                       deps.Tangle.Synced(),
		MetricsLevel:                 Parameters.MetricsLevel,
		BufferSize:                   uint32(scheduler.BufferSize()),
		BufferLength:                 uint32(scheduler.TotalMessagesCount()),
		ReadyMessagesInBuffer:        uint32(scheduler.ReadyMessagesCount()),
		QueueLengthPerNode:           queueMap,
		AManaNormalizedLengthPerNode: aManaNormalizedMap,
		Timestamp:                    timestamp,
	}

	_ = deps.RemoteLogger.Send(record)
}

func prepQueueMaps(s *tangle.Scheduler) (queueMap map[string]uint32, aManaNormalizedMap map[string]float64) {
	queueSizes := s.NodeQueueSizes()
	queueMap = make(map[string]uint32, len(queueSizes))
	aManaNormalizedMap = make(map[string]float64, len(queueSizes))

	for id, size := range queueSizes {
		nodeID := id.String()
		aMana := s.GetManaFromCache(id)

		queueMap[nodeID] = uint32(size)
		aManaNormalizedMap[nodeID] = float64(size) / aMana
	}
	return
}
