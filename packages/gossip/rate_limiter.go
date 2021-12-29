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

type RateLimit struct {
	Limit    int
	Interval time.Duration
}

func (rl RateLimit) String() string {
	return fmt.Sprintf("%d per %s", rl.Limit, rl.Interval)
}

type limiterRecord struct {
	counter          *ratecounter.RateCounter
	limitHitReported *atomic.Bool
}

type neighborRateLimiter struct {
	name             string
	conf             *RateLimit
	hitEvent         map[NeighborsGroup]*events.Event
	log              *logger.Logger
	neighborsRecords *ttlcache.Cache
}

func newNeighborRateLimiter(name string, limit *RateLimit, hitEvent map[NeighborsGroup]*events.Event) (*neighborRateLimiter, error) {
	records := ttlcache.NewCache()
	records.SetLoaderFunction(func(_ string) (interface{}, time.Duration, error) {
		record := &limiterRecord{counter: ratecounter.NewRateCounter(limit.Interval), limitHitReported: atomic.NewBool(false)}
		return record, ttlcache.ItemExpireWithGlobalTTL, nil
	})
	if err := records.SetTTL(limit.Interval); err != nil {
		return nil, errors.WithStack(err)
	}
	return &neighborRateLimiter{
		name:             name,
		conf:             limit,
		hitEvent:         hitEvent,
		neighborsRecords: records,
	}, nil
}

func (nrl *neighborRateLimiter) count(nbr *Neighbor) {
	if err := nrl.doCount(nbr); err != nil {
		nbr.log.Warnw("Rate limiter failed count to neighbor activity", "rateLimiter", nrl.name)
	}
}

func (nrl *neighborRateLimiter) doCount(nbr *Neighbor) error {
	nbrKey := getNeighborKey(nbr)
	nbrRecordI, err := nrl.neighborsRecords.Get(nbrKey)
	if err != nil {
		return errors.WithStack(err)
	}
	nbrRecord := nbrRecordI.(*limiterRecord)
	nbrRecord.counter.Incr(1)
	if int(nbrRecord.counter.Rate()) > nrl.conf.Limit {
		if !nbrRecord.limitHitReported.Swap(true) {
			nbr.log.Infow("Neighbor hit the activity limit, notifying subscribers to take action",
				"rateLimiter", nrl.name, "limit", nrl.conf)
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
