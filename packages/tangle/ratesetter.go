package tangle

import (
	"math"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/tangle/schedulerutils"
)

const (
	// MaxLocalQueueSize is the maximum local (containing the message to be issued) queue size in bytes
	MaxLocalQueueSize = 20 * MaxMessageSize
	// Backoff is the local threshold for rate setting; < MaxQueueWeight
	Backoff = 25.0
	// RateSettingIncrease is the global additive increase parameter.
	RateSettingIncrease = 1.0
	// RateSettingDecrease global multiplicative decrease parameter  (larger than 1)
	RateSettingDecrease = 1.5
	// RateSettingPause is the time to wait before next rate's update after a backoff
	RateSettingPause = 2
)

var (
	// ErrInvalidIssuer is returned when an invalid message is passed to the rate setter.
	ErrInvalidIssuer = errors.New("message not issued by local node")
	// ErrStopped is returned when a message is passed to a stopped rate setter.
	ErrStopped = errors.New("rate setter stopped")
)

// Initial is the rate in bytes per second
var Initial = 20000.0

// region RateSetterParams /////////////////////////////////////////////////////////////////////////////////////////////

// RateSetterParams represents the parameters for RateSetter.
type RateSetterParams struct {
	Initial *float64
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RateSetter /////////////////////////////////////////////////////////////////////////////////////////////

// RateSetter is a Tangle component that takes care of congestion control of local node.
type RateSetter struct {
	tangle         *Tangle
	Events         *RateSetterEvents
	self           identity.ID
	issuingQueue   *schedulerutils.NodeQueue
	issueChan      chan *Message
	ownRate        *atomic.Float64
	pauseUpdates   uint
	shutdownSignal chan struct{}
	shutdownOnce   sync.Once
}

// NewRateSetter returns a new RateSetter.
func NewRateSetter(tangle *Tangle) *RateSetter {
	if tangle.Options.RateSetterParams.Initial != nil {
		Initial = *tangle.Options.RateSetterParams.Initial
	}

	rateSetter := &RateSetter{
		tangle: tangle,
		Events: &RateSetterEvents{
			MessageDiscarded: events.NewEvent(MessageIDCaller),
			MessageIssued:    events.NewEvent(MessageCaller),
			Error:            events.NewEvent(events.ErrorCaller),
		},
		self:           tangle.Options.Identity.ID(),
		issuingQueue:   schedulerutils.NewNodeQueue(tangle.Options.Identity.ID()),
		issueChan:      make(chan *Message),
		ownRate:        atomic.NewFloat64(Initial),
		pauseUpdates:   0,
		shutdownSignal: make(chan struct{}),
		shutdownOnce:   sync.Once{},
	}

	go rateSetter.issuerLoop()
	return rateSetter
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (r *RateSetter) Setup() {
	r.tangle.MessageFactory.Events.MessageConstructed.Attach(events.NewClosure(func(msg *Message) {
		if err := r.Issue(msg); err != nil {
			r.Events.Error.Trigger(errors.Errorf("failed to submit to rate setter: %w", err))
		}
	}))
	// update own rate setting
	r.tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID MessageID) {
		if r.pauseUpdates > 0 {
			r.pauseUpdates--
			return
		}
		if r.issuingQueue.Size() > 0 {
			r.rateSetting()
		}
	}))
}

// Issue submits a message to the local issuing queue.
func (r *RateSetter) Issue(message *Message) error {
	if identity.NewID(message.IssuerPublicKey()) != r.self {
		return ErrInvalidIssuer
	}

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
	// TODO: https://github.com/iotaledger/goshimmer/issues/1483

	// dummy estimate
	return time.Duration(math.Ceil(float64(r.Size()) / r.ownRate.Load() * float64(time.Second)))
}

// rateSetting updates the rate ownRate at which messages can be issued by the node.
func (r *RateSetter) rateSetting() {
	ownMana := r.tangle.Options.SchedulerParams.AccessManaRetrieveFunc(r.self)
	totalMana := r.tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc()

	ownRate := r.ownRate.Load()
	if float64(r.tangle.Scheduler.NodeQueueSize(r.self))/ownMana > Backoff {
		ownRate /= RateSettingDecrease
		r.pauseUpdates = RateSettingPause
	} else {
		ownRate += RateSettingIncrease * ownMana / totalMana
	}
	r.ownRate.Store(ownRate)
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
			r.Events.MessageIssued.Trigger(msg)
			lastIssueTime = time.Now()

			if next := r.issuingQueue.Front(); next != nil {
				issueTimer.Reset(time.Until(lastIssueTime.Add(r.issueInterval(next.(*Message)))))
				timerStopped = false
			}

		// add a new message to the local issuer queue
		case msg := <-r.issueChan:
			if r.issuingQueue.Size()+len(msg.Payload().Bytes()) > MaxLocalQueueSize {
				r.Events.MessageDiscarded.Trigger(msg.ID())
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
				issueTimer.Reset(time.Until(lastIssueTime.Add(r.issueInterval(next.(*Message)))))
			}

		// on close, exit the loop
		case <-r.shutdownSignal:
			break loop
		}
	}

	// discard all remaining messages at shutdown
	for _, id := range r.issuingQueue.IDs() {
		r.Events.MessageDiscarded.Trigger(id)
	}
}

func (r *RateSetter) issueInterval(msg *Message) time.Duration {
	wait := time.Duration(math.Ceil(float64(len(msg.Payload().Bytes())) / r.ownRate.Load() * float64(time.Second)))
	return wait
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RateSetterEvents /////////////////////////////////////////////////////////////////////////////////////////////

// RateSetterEvents represents events happening in the rate setter.
type RateSetterEvents struct {
	MessageDiscarded *events.Event
	MessageIssued    *events.Event
	Error            *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
