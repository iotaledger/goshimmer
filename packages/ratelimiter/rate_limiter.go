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

// RateLimit contains information about rate limit values such as time interval and the limit.
type RateLimit struct {
	Interval time.Duration
	Limit    int
}

// String return a string representation of the RateLimit instance.
func (rl RateLimit) String() string {
	return fmt.Sprintf("%d per %s", rl.Limit, rl.Interval)
}

// PeerRateLimiter is an object to count activity of peers
// and notify the subscribers in case the limit of activity is exceeded.
type PeerRateLimiter struct {
	interval     time.Duration
	baseLimit    *atomic.Int64
	hitEvent     *events.Event
	peersRecords *ttlcache.Cache
	log          *logger.Logger
}

// NewPeerRateLimiter returns a new instance of the PeerRateLimiter object.
func NewPeerRateLimiter(interval time.Duration, baseLimit int, log *logger.Logger) (*PeerRateLimiter, error) {
	records := ttlcache.NewCache()
	records.SetLoaderFunction(func(_ string) (interface{}, time.Duration, error) {
		record := &limiterRecord{
			activityCounter:       ratecounter.NewRateCounter(interval),
			limitExtensionCounter: ratecounter.NewRateCounter(interval),
			limitHitReported:      atomic.NewBool(false),
		}

		return record, ttlcache.ItemExpireWithGlobalTTL, nil
	})
	if err := records.SetTTL(interval); err != nil {
		return nil, errors.WithStack(err)
	}
	return &PeerRateLimiter{
		interval:     interval,
		baseLimit:    atomic.NewInt64(int64(baseLimit)),
		hitEvent:     events.NewEvent(limitHitCaller),
		peersRecords: records,
		log:          log,
	}, nil
}

type limiterRecord struct {
	activityCounter       *ratecounter.RateCounter
	limitExtensionCounter *ratecounter.RateCounter
	limitHitReported      *atomic.Bool
}

// Count counts a new activity of the peer towards its rate limit.
func (prl *PeerRateLimiter) Count(p *peer.Peer) {
	if err := prl.doCount(p); err != nil {
		prl.log.Warnw("Rate limiter failed to count peer activity",
			"peerId", p.ID())
	}
}

// ExtendLimit extends the activity limit of the peer.
func (prl *PeerRateLimiter) ExtendLimit(p *peer.Peer, val int) {
	if err := prl.doExtendLimit(p, val); err != nil {
		prl.log.Warnw("Rate limiter failed to extend peer activity limit",
			"peerId", p.ID())
	}
}

// SetBaseLimit updates the value of the base limit.
func (prl *PeerRateLimiter) SetBaseLimit(limit int) {
	prl.baseLimit.Store(int64(limit))
}

// HitEvent returns the event instance which is triggered when a peer exceeds the activity limit.
func (prl *PeerRateLimiter) HitEvent() *events.Event {
	return prl.hitEvent
}

// Close closes PeerRateLimiter instance, it can't be used after that.
func (prl *PeerRateLimiter) Close() {
	if err := prl.peersRecords.Close(); err != nil {
		prl.log.Errorw("Failed to close peers records cache", "err", err)
	}
}

func (prl *PeerRateLimiter) doCount(p *peer.Peer) error {
	peerRecord, err := prl.getPeerRecord(p)
	if err != nil {
		return errors.WithStack(err)
	}
	peerRecord.activityCounter.Incr(1)
	limit := int(prl.baseLimit.Load() + peerRecord.limitExtensionCounter.Rate())
	if int(peerRecord.activityCounter.Rate()) > limit {
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

func (prl *PeerRateLimiter) doExtendLimit(p *peer.Peer, val int) error {
	peerRecord, err := prl.getPeerRecord(p)
	if err != nil {
		return errors.WithStack(err)
	}
	peerRecord.limitExtensionCounter.Incr(int64(val))
	return nil
}

func (prl *PeerRateLimiter) getPeerRecord(p *peer.Peer) (*limiterRecord, error) {
	peerKey := p.ID().EncodeBase58()
	nbrRecordI, err := prl.peersRecords.Get(peerKey)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	peerRecord := nbrRecordI.(*limiterRecord)
	return peerRecord, nil
}

func limitHitCaller(handler interface{}, params ...interface{}) {
	handler.(func(*peer.Peer, *RateLimit))(params[0].(*peer.Peer), params[1].(*RateLimit))
}
