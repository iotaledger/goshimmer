package tangle

import (
	"math"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/identity"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/tangle/schedulerutils"
)

const (
	// MaxLocalQueueSize is the maximum local (containing the message to be issued) queue size in bytes.
	MaxLocalQueueSize = 20
	// RateSettingIncrease is the global additive increase parameter.
	// In the Access Control for Distributed Ledgers in the Internet of Things: A Networking Approach paper this parameter is denoted as "A".
	RateSettingIncrease = 0.075
	// RateSettingDecrease global multiplicative decrease parameter (larger than 1).
	// In the Access Control for Distributed Ledgers in the Internet of Things: A Networking Approach paper this parameter is denoted as "β".
	RateSettingDecrease = 1.5
	// Wmax is the maximum inbox threshold for the node. This value denotes when maximum active mana holder backs off its rate.
	Wmax = 10
	// Wmin is the min inbox threshold for the node. This value denotes when nodes with the least mana back off their rate.
	Wmin = 5
)

var (
	// ErrInvalidIssuer is returned when an invalid message is passed to the rate setter.
	ErrInvalidIssuer = errors.New("message not issued by local node")
	// ErrStopped is returned when a message is passed to a stopped rate setter.
	ErrStopped = errors.New("rate setter stopped")
)

// region RateSetterParams /////////////////////////////////////////////////////////////////////////////////////////////

// RateSetterParams represents the parameters for RateSetter.
type RateSetterParams struct {
	Enabled          bool
	Initial          float64
	RateSettingPause time.Duration
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

// RateSetter is a Tangle component that takes care of congestion control of local node.
type RateSetter struct {
	tangle              *Tangle
	Events              *RateSetterEvents
	self                identity.ID
	issuingQueue        *schedulerutils.NodeQueue
	issueChan           chan *Message
	ownRate             *atomic.Float64
	pauseUpdates        uint
	initialPauseUpdates uint
	maxRate             float64
	shutdownSignal      chan struct{}
	shutdownOnce        sync.Once
	initOnce            sync.Once
}

// NewRateSetter returns a new RateSetter.
func NewRateSetter(tangle *Tangle) *RateSetter {

	rateSetter := &RateSetter{
		tangle:              tangle,
		Events:              newRateSetterEvents(),
		self:                tangle.Options.Identity.ID(),
		issuingQueue:        schedulerutils.NewNodeQueue(tangle.Options.Identity.ID()),
		issueChan:           make(chan *Message),
		ownRate:             atomic.NewFloat64(0),
		pauseUpdates:        0,
		initialPauseUpdates: uint(tangle.Options.RateSetterParams.RateSettingPause / tangle.Scheduler.Rate()),
		maxRate:             float64(time.Second / tangle.Scheduler.Rate()),
		shutdownSignal:      make(chan struct{}),
		shutdownOnce:        sync.Once{},
	}

	go rateSetter.issuerLoop()
	return rateSetter
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (r *RateSetter) Setup() {
	if !r.tangle.Options.RateSetterParams.Enabled {
		r.tangle.MessageFactory.Events.MessageConstructed.Attach(event.NewClosure(func(event *MessageConstructedEvent) {
			r.Events.MessageIssued.Trigger(event)
		}))
		return
	}

	r.tangle.MessageFactory.Events.MessageConstructed.Attach(event.NewClosure(func(event *MessageConstructedEvent) {
		if err := r.Issue(event.Message); err != nil {
			r.Events.Error.Trigger(errors.Errorf("failed to submit to rate setter: %w", err))
		}
	}))
	// update own rate setting
	r.tangle.Scheduler.Events.MessageScheduled.Attach(event.NewClosure(func(_ *MessageScheduledEvent) {
		if r.pauseUpdates > 0 {
			r.pauseUpdates--
			return
		}
		if r.issuingQueue.Size()+r.tangle.Scheduler.NodeQueueSize(selfLocalIdentity.ID()) > 0 {
			r.rateSetting()
		}
	}))
}

// Issue submits a message to the local issuing queue.
func (r *RateSetter) Issue(message *Message) error {
	if identity.NewID(message.IssuerPublicKey()) != r.self {
		return ErrInvalidIssuer
	}
	r.initOnce.Do(func() {
		// initialize mana vectors in cache here when mana vectors are already loaded
		r.initializeInitialRate()
	})

	select {
	case r.issueChan <- message:
		return nil
	case <-r.shutdownSignal:
		return ErrStopped
	}
}

// Shutdown shuts down the RateSetter.
func (r *RateSetter) Shutdown() {
	r.shutdownOnce.Do(func() {
		close(r.shutdownSignal)
	})
}

// Rate returns the rate of the rate setter.
func (r *RateSetter) Rate() float64 {
	return r.ownRate.Load()
}

// Size returns the size of the issuing queue.
func (r *RateSetter) Size() int {
	return r.issuingQueue.Size()
}

// Estimate estimates the issuing time of new message.
func (r *RateSetter) Estimate() time.Duration {
	r.initOnce.Do(func() {
		// initialize mana vectors in cache here when mana vectors are already loaded
		r.initializeInitialRate()
	})

	var pauseUpdate time.Duration
	if r.pauseUpdates > 0 {
		pauseUpdate = time.Duration(float64(r.pauseUpdates)) * r.tangle.Scheduler.Rate()
	}
	// dummy estimate
	return lo.Max(time.Duration(math.Ceil(float64(r.Size())/r.ownRate.Load()*float64(time.Second))), pauseUpdate)
}

// rateSetting updates the rate ownRate at which messages can be issued by the node.
func (r *RateSetter) rateSetting() {
	// Return access mana or MinMana to allow zero mana nodes issue.
	ownMana := math.Max(r.tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc()[r.self], MinMana)
	maxManaValue := lo.Max(append(lo.Values(r.tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc()), MinMana)...)
	totalMana := r.tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc()

	ownRate := r.ownRate.Load()

	// TODO: make sure not to issue or scheduled blocks older than TSC
	if float64(r.tangle.Scheduler.NodeQueueSize(r.self)) > math.Max(Wmin, Wmax*ownMana/maxManaValue) {
		ownRate /= RateSettingDecrease
		r.pauseUpdates = r.initialPauseUpdates
	} else {
		ownRate += math.Max(RateSettingIncrease*ownMana/totalMana, 0.01)
	}
	r.ownRate.Store(math.Min(r.maxRate, ownRate))
}

func (r *RateSetter) issuerLoop() {
	var (
		issueTimer    = time.NewTimer(0) // setting this to 0 will cause a trigger right away
		timerStopped  = false
		lastIssueTime = time.Now()
	)
	defer issueTimer.Stop()

loop:
	for {
		select {
		// a new message can be submitted to the scheduler
		case <-issueTimer.C:
			timerStopped = true
			if r.issuingQueue.Front() == nil {
				continue
			}

			msg := r.issuingQueue.PopFront().(*Message)
			r.Events.MessageIssued.Trigger(&MessageConstructedEvent{Message: msg})
			lastIssueTime = time.Now()

			if next := r.issuingQueue.Front(); next != nil {
				issueTimer.Reset(time.Until(lastIssueTime.Add(r.issueInterval())))
				timerStopped = false
			}

		// add a new message to the local issuer queue
		case msg := <-r.issueChan:
			if r.issuingQueue.Size()+1 > MaxLocalQueueSize {
				r.Events.MessageDiscarded.Trigger(&MessageDiscardedEvent{msg.ID()})
				continue
			}
			// add to queue
			r.issuingQueue.Submit(msg)
			r.issuingQueue.Ready(msg)

			// set a new timer if needed
			// if a timer is already running it is not updated, even if the ownRate has changed
			if !timerStopped {
				break
			}
			if next := r.issuingQueue.Front(); next != nil {
				issueTimer.Reset(time.Until(lastIssueTime.Add(r.issueInterval())))
			}

		// on close, exit the loop
		case <-r.shutdownSignal:
			break loop
		}
	}

	// discard all remaining messages at shutdown
	for _, id := range r.issuingQueue.IDs() {
		r.Events.MessageDiscarded.Trigger(&MessageDiscardedEvent{NewMessageID(id)})
	}
}

func (r *RateSetter) issueInterval() time.Duration {
	wait := time.Duration(math.Ceil(float64(1) / r.ownRate.Load() * float64(time.Second)))
	return wait
}

func (r *RateSetter) initializeInitialRate() {
	if r.tangle.Options.RateSetterParams.Initial > 0.0 {
		r.ownRate.Store(r.tangle.Options.RateSetterParams.Initial)
	} else {
		ownMana := math.Max(r.tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc()[r.tangle.Options.Identity.ID()], MinMana)
		totalMana := r.tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc()
		r.ownRate.Store(math.Max(ownMana/totalMana*r.maxRate, 1))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RateSetterEvents /////////////////////////////////////////////////////////////////////////////////////////////

// RateSetterEvents represents events happening in the rate setter.
type RateSetterEvents struct {
	MessageDiscarded *event.Event[*MessageDiscardedEvent]
	MessageIssued    *event.Event[*MessageConstructedEvent]
	Error            *event.Event[error]
}

func newRateSetterEvents() (new *RateSetterEvents) {
	return &RateSetterEvents{
		MessageDiscarded: event.New[*MessageDiscardedEvent](),
		MessageIssued:    event.New[*MessageConstructedEvent](),
		Error:            event.New[error](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
