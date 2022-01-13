package gossip

import (
	"fmt"
	"time"

	"github.com/ReneKroon/ttlcache/v2"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/paulbellamy/ratecounter"
	"go.uber.org/atomic"
)

type limit interface {
	limit() int
	extend()
}

//type StaticLimit struct {
//	limit int
//}
//
//func NewStaticLimit(limit int) *StaticLimit { return &StaticLimit{limit: limit} }
//
//func (sl *StaticLimit) limit() int { return sl.limit }

type dynamicLimit struct {
	baseLimit        int
	extensionCounter *ratecounter.RateCounter
}

func newDynamicLimit(baseLimit int, extensionInterval time.Duration) *dynamicLimit {
	return &dynamicLimit{
		baseLimit:        baseLimit,
		extensionCounter: ratecounter.NewRateCounter(extensionInterval),
	}
}

func (dl *dynamicLimit) limit() int {
	return dl.baseLimit + int(dl.extensionCounter.Rate())
}

func (dl *dynamicLimit) extend() {
	dl.extensionCounter.Incr(1)
}

type RateLimitConfig struct {
	Interval               time.Duration
	Limit                  int
	LimitExtensionInterval time.Duration
}

type limiterRecord struct {
	counter          *ratecounter.RateCounter
	limitHitReported *atomic.Bool
}

type neighborRateLimiter struct {
	name             string
	limit            limit
	interval         time.Duration
	hitEvent         map[NeighborsGroup]*events.Event
	log              *logger.Logger
	neighborsRecords *ttlcache.Cache
}

func newNeighborRateLimiter(name string, conf *RateLimitConfig, hitEvent map[NeighborsGroup]*events.Event) (*neighborRateLimiter, error) {
	records := ttlcache.NewCache()
	records.SetLoaderFunction(func(_ string) (interface{}, time.Duration, error) {
		record := &limiterRecord{counter: ratecounter.NewRateCounter(conf.Interval), limitHitReported: atomic.NewBool(false)}
		return record, ttlcache.ItemExpireWithGlobalTTL, nil
	})
	if err := records.SetTTL(conf.Interval); err != nil {
		return nil, errors.WithStack(err)
	}
	return &neighborRateLimiter{
		name:             name,
		interval:         conf.Interval,
		limit:            newDynamicLimit(conf.Limit, conf.LimitExtensionInterval),
		hitEvent:         hitEvent,
		neighborsRecords: records,
	}, nil
}

func (nrl *neighborRateLimiter) count(nbr *Neighbor) {
	if err := nrl.doCount(nbr); err != nil {
		nbr.log.Warnw("Rate limiter failed count to neighbor activity", "rateLimiter", nrl.name)
	}
}

func (nrl *neighborRateLimiter) extendLimit() {
	nrl.limit.extend()
}

func (nrl *neighborRateLimiter) doCount(nbr *Neighbor) error {
	nbrKey := getNeighborKey(nbr)
	nbrRecordI, err := nrl.neighborsRecords.Get(nbrKey)
	if err != nil {
		return errors.WithStack(err)
	}
	nbrRecord := nbrRecordI.(*limiterRecord)
	nbrRecord.counter.Incr(1)
	limitValue := nrl.limit.limit()
	if int(nbrRecord.counter.Rate()) > limitValue {
		if !nbrRecord.limitHitReported.Swap(true) {
			nbr.log.Infow("Neighbor hit the activity limit, notifying subscribers to take action",
				"rateLimiter", nrl.name, "limit", limitValue, "interval", nrl.interval)
			nrl.hitEvent[nbr.Group].Trigger(nbr)
		}
	} else {
		nbrRecord.limitHitReported.Store(false)
	}
	return nil
}

func (nrl *neighborRateLimiter) close() error {
	return errors.WithStack(nrl.neighborsRecords.Close())
}

func getNeighborKey(nbr *Neighbor) string {
	return fmt.Sprintf("%s;group=%d", nbr.ID(), nbr.Group)
}
