package scheduler

//// Scheduler is a Tangle component that takes care of scheduling the blocks that shall be booked.
//type Scheduler struct {
//	Events *Events
//
//	ticker          *time.Ticker
//	accessManaCache *schedulerutils.AccessManaCache
//	bufferMutex     sync.RWMutex
//	buffer          *schedulerutils.BufferQueue
//	deficitsMutex   sync.RWMutex
//	deficits        map[identity.ID]*big.Rat
//
//	rate                  *atomic.Duration
//	confirmedBlkThreshold time.Duration
//
//	shutdownSignal chan struct{}
//	shutdownOnce   sync.Once
//}
