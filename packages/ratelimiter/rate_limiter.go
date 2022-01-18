package ratelimiter

import (
	"fmt"
	"time"

	"github.com/ReneKroon/ttlcache/v2"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/paulbellamy/ratecounter"
	"go.uber.org/atomic"
)

type RateLimit struct {
	Interval time.Duration
	Limit    int
}

func (rl RateLimit) String() string {
	return fmt.Sprintf("%d per %s", rl.Limit, rl.Interval)
}

type PeerRateLimiter struct {
	interval     time.Duration
	limit        *atomic.Int64
	hitEvent     *events.Event
	peersRecords *ttlcache.Cache
	log          *logger.Logger
}

func NewPeerRateLimiter(interval time.Duration, limit int, log *logger.Logger) (*PeerRateLimiter, error) {
	records := ttlcache.NewCache()
	records.SetLoaderFunction(func(_ string) (interface{}, time.Duration, error) {
		record := &limiterRecord{counter: ratecounter.NewRateCounter(interval), limitHitReported: atomic.NewBool(false)}
		return record, ttlcache.ItemExpireWithGlobalTTL, nil
	})
	if err := records.SetTTL(interval); err != nil {
		return nil, errors.WithStack(err)
	}
	return &PeerRateLimiter{
		interval:     interval,
		limit:        atomic.NewInt64(int64(limit)),
		hitEvent:     events.NewEvent(limitHitCaller),
		peersRecords: records,
		log:          log,
	}, nil
}

type limiterRecord struct {
	counter          *ratecounter.RateCounter
	limitHitReported *atomic.Bool
}

func (prl *PeerRateLimiter) Count(p *peer.Peer) {
	if err := prl.doCount(p); err != nil {
		prl.log.Warnw("Rate limiter failed to count peer activity",
			"peerId", p.ID())
	}
}

func (prl *PeerRateLimiter) SetLimit(limit int) {
	prl.limit.Store(int64(limit))
}

func (prl *PeerRateLimiter) HitEvent() *events.Event {
	return prl.hitEvent
}

func (prl *PeerRateLimiter) Close() {
	if err := prl.peersRecords.Close(); err != nil {
		prl.log.Errorw("Failed to close peers records cache", "err", err)
	}
}
func (prl *PeerRateLimiter) doCount(p *peer.Peer) error {
	peerKey := p.ID().String()
	nbrRecordI, err := prl.peersRecords.Get(peerKey)
	if err != nil {
		return errors.WithStack(err)
	}
	peerRecord := nbrRecordI.(*limiterRecord)
	peerRecord.counter.Incr(1)
	limit := int(prl.limit.Load())
	if int(peerRecord.counter.Rate()) > limit {
		if !peerRecord.limitHitReported.Swap(true) {
			prl.log.Infow("Peer hit the activity limit, notifying subscribers to take action",
				"limit", limit, "interval", prl.interval, "peerId", p.ID())
			prl.hitEvent.Trigger(p, &RateLimit{Limit: limit, Interval: prl.interval})
		}
	} else {
		peerRecord.limitHitReported.Store(false)
	}
	return nil
}

func limitHitCaller(handler interface{}, params ...interface{}) {
	handler.(func(*peer.Peer, *RateLimit))(params[0].(*peer.Peer), params[1].(*RateLimit))
}
