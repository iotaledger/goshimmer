package remotemetrics

import (
	"time"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/remotemetrics"
)

func obtainSchedulerStats(timestamp time.Time) {
	scheduler := deps.Tangle.Scheduler
	record := remotemetrics.SchedulerMetrics{
		Type:                  "schedulerSample",
		MetricsLevel:          Parameters.MetricsLevel,
		BufferSize:            uint32(scheduler.BufferSize()),
		BufferLength:          uint32(scheduler.TotalMessagesCount()),
		ReadyMessagesInBuffer: uint32(scheduler.ReadyMessagesCount()),
		QueueLengthPerNode:    prepQueueMap(scheduler.NodeQueueSizes()),
		Timestamp:             timestamp,
	}

	if err := deps.RemoteLogger.Send(record); err != nil {
		Plugin.Logger().Errorw("Failed to send "+record.Type+" record", "err", err)
	}
}

func prepQueueMap(queueSizes map[identity.ID]int) map[string]uint32 {
	newMap := make(map[string]uint32, len(queueSizes))
	for id, size := range queueSizes {
		newMap[id.String()] = uint32(size)
	}
	return newMap
}
