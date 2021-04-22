package tangle

import (
	"math"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"go.uber.org/atomic"
	"golang.org/x/xerrors"
)

const (
	// MaxLocalQueueSize is the maximum local (containing the message to be issued) queue size in bytes
	MaxLocalQueueSize = 20 * MaxMessageSize
	// Backoff is the local threshold for rate setting; < MaxQueueWeight
	Backoff = 25.0
	// A is the additive increase
	A = 1.0
	// Tau is the time to wait before next rate's update after a backoff
	Tau = 2
)

var (
	// Initial is the rate in bytes per second
	Initial = 20000.0
	// Beta is the multiplicative decrease
	Beta = 0.7
)

// region RateSetterParams /////////////////////////////////////////////////////////////////////////////////////////////

type RateSetterParams struct {
	Beta    *float64
	Initial *float64
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RateSetter /////////////////////////////////////////////////////////////////////////////////////////////

type RateSetter struct {
	tangle         *Tangle
	Events         *RateSetterEvents
	self           identity.ID
	mu             sync.Mutex
	issuingQueue   *NodeQueue
	issue          chan *Message
	lambda         *atomic.Float64
	haltUpdate     uint
	shutdownSignal chan struct{}
	shutdownOnce   sync.Once
}

func NewRateSetter(tangle *Tangle) *RateSetter {
	rateSetter := &RateSetter{
		tangle: tangle,
		Events: &RateSetterEvents{
			MessageDiscarded: events.NewEvent(MessageIDCaller),
			MessageIssued:    events.NewEvent(messageEventHandler),
		},
		self:           tangle.Options.Identity.ID(),
		issuingQueue:   NewNodeQueue(tangle.Options.Identity.ID()),
		issue:          make(chan *Message, 1),
		lambda:         atomic.NewFloat64(Initial),
		haltUpdate:     0,
		shutdownSignal: make(chan struct{}),
		shutdownOnce:   sync.Once{},
	}
	if tangle.Options.RateSetterParams.Initial != nil {
		Initial = *tangle.Options.RateSetterParams.Initial
	}
	if tangle.Options.RateSetterParams.Beta != nil {
		Beta = *tangle.Options.RateSetterParams.Beta
	}

	go rateSetter.issuerLoop()
	return rateSetter
}

func (r *RateSetter) Setup() {
	r.tangle.MessageFactory.Events.MessageConstructed.Attach(events.NewClosure(r.submit))
	r.tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID MessageID) {
		r.rateSetting()
	}))
}

func (r *RateSetter) unsubmit(message *Message) {
	r.issuingQueue.Unsubmit(message)
	r.tangle.Scheduler.Unsubmit(message.ID())
}

func (r *RateSetter) submit(message *Message) {
	if r.issuingQueue.Size()+uint(len(message.Bytes())) > MaxLocalQueueSize {
		r.Events.MessageDiscarded.Trigger(message.ID())
		return
	}
	submitted, err := r.issuingQueue.Submit(message)
	if err != nil {
		r.tangle.Events.Error.Trigger(xerrors.Errorf("failed to submit message to issuing queue: %w", err))
		return
	}
	if submitted {
		r.issue <- message
	}
}

func (r *RateSetter) Shutdown() {
	r.shutdownOnce.Do(func() {
		close(r.shutdownSignal)
	})
}

func (r *RateSetter) rateSetting() {
	if r.haltUpdate > 0 {
		r.haltUpdate--
		return
	}

	mana := r.tangle.Options.SchedulerParams.AccessManaRetrieveFunc(r.self)
	totalMana := r.tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc()
	if mana <= 0 {
		r.tangle.Events.Error.Trigger(xerrors.Errorf("invalid mana: %f", mana))
		return
	}

	lambda := r.lambda.Load()
	if float64(r.tangle.Scheduler.NodeQueueSize(r.self))/mana > Backoff {
		lambda *= Beta
		r.haltUpdate = Tau
	} else {
		lambda += A * mana / totalMana
	}
	r.lambda.Store(lambda)
}

func (r *RateSetter) issuerLoop() {
	var (
		issue         = time.NewTimer(0) // setting this to 0 will cause a trigger right away
		timerStopped  = false
		lastIssueTime = time.Now()
	)
	defer issue.Stop()

	for {
		select {
		// a new message can be scheduled
		case <-issue.C:
			timerStopped = true
			if !r.issueNext() {
				continue
			}
			lastIssueTime = time.Now()

			if next := r.issuingQueue.Front(); next != nil {
				issue.Reset(time.Until(lastIssueTime.Add(r.issueInterval(next))))
				timerStopped = false
			}

		// add a new message to the local issuer queue
		case msg := <-r.issue:
			nodeID := identity.NewID(msg.IssuerPublicKey())
			if nodeID != r.self {
				panic("invalid message")
			}
			// mark the message as ready so that it will be picked up at the next tick
			func() {
				r.mu.Lock()
				defer r.mu.Unlock()
				r.issuingQueue.Ready(msg)
			}()

			// set a new timer if needed
			// if a timer is already running it is not updated, even if the lambda has changed
			if !timerStopped {
				break
			}
			if next := r.issuingQueue.Front(); next != nil {
				issue.Reset(time.Until(lastIssueTime.Add(r.issueInterval(next))))
			}

		// on close, exit the loop
		case <-r.shutdownSignal:
			return
		}
	}
}

func (r *RateSetter) issueNext() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	msg := r.issuingQueue.Front()
	if msg == nil {
		return false
	}
	r.Events.MessageIssued.Trigger(msg)
	r.issuingQueue.PopFront()
	return true
}

func (r *RateSetter) issueInterval(msg *Message) time.Duration {
	wait := time.Duration(math.Ceil(float64(len(msg.Bytes())) / r.lambda.Load() * float64(time.Second)))
	return wait
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RateSetterEvents /////////////////////////////////////////////////////////////////////////////////////////////

// RateSetterEvents represents events happening in the rate setter.
type RateSetterEvents struct {
	MessageDiscarded *events.Event
	MessageIssued    *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
