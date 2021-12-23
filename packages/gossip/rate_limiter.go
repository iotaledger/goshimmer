package gossip

import (
	"fmt"
	"time"

	"github.com/ReneKroon/ttlcache/v2"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/events"
	"github.com/paulbellamy/ratecounter"
	"go.uber.org/atomic"
)

type RateLimiterConf struct {
	Interval time.Duration
	Limit    int
}

type limiterRecord struct {
	counter          *ratecounter.RateCounter
	limitHitReported *atomic.Bool
}

type neighborRateLimiter struct {
	name             string
	conf             *RateLimiterConf
	event            *events.Event
	neighborsRecords *ttlcache.Cache
}

func newNeighborRateLimiter(name string, conf *RateLimiterConf, event *events.Event) (*neighborRateLimiter, error) {
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
		conf:             conf,
		event:            event,
		neighborsRecords: records,
	}, nil
}

func (nrl *neighborRateLimiter) Count(nbr *Neighbor) {
	if err := nrl.count(nbr); err != nil {
		nbr.log.Warnw("Rate limiter failed to neighbor activity", "rateLimiter", nrl.name)
	}
}

func (nrl *neighborRateLimiter) count(nbr *Neighbor) error {
	nbrKey := getNeighborKey(nbr)
	nbrRecordI, err := nrl.neighborsRecords.Get(nbrKey)
	if err != nil {
		return errors.WithStack(err)
	}
	nbrRecord := nbrRecordI.(*limiterRecord)
	nbrRecord.counter.Incr(1)
	if int(nbrRecord.counter.Rate()) > nrl.conf.Limit {
		if !nbrRecord.limitHitReported.Swap(true) {
			nrl.event.Trigger(nbr)
		}
	} else {
		nbrRecord.limitHitReported.Store(false)
	}
	return nil
}

func getNeighborKey(nbr *Neighbor) string {
	return fmt.Sprintf("%s;group=%d", nbr.ID(), nbr.Group)
}
