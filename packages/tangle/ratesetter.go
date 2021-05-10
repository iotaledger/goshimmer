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
	MaxLocalQueueSize = 10000 * MaxMessageSize //TODO: 20
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
	Beta          = 0.7
	issueChanSize = 1000
)

// region RateSetterParams /////////////////////////////////////////////////////////////////////////////////////////////

// RateSetterParams represents the parameters for RateSetter.
type RateSetterParams struct {
	Beta    *float64
	Initial *float64
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RateSetter /////////////////////////////////////////////////////////////////////////////////////////////

// RateSetter is a Tangle component that takes care of congestion control of local node.
type RateSetter struct {
	tangle         *Tangle
	Events         *RateSetterEvents
	self           identity.ID
	mu             sync.Mutex
	issuingQueue   *schedulerutils.NodeQueue
	issue          chan *Message
	lambda         *atomic.Float64
	haltUpdate     uint
	shutdownSignal chan struct{}
	shutdownOnce   sync.Once
}

// NewRateSetter returns a new RateSetter.
func NewRateSetter(tangle *Tangle) *RateSetter {
	rateSetter := &RateSetter{
		tangle: tangle,
		Events: &RateSetterEvents{
			MessageDiscarded: events.NewEvent(MessageIDCaller),
			MessageIssued:    events.NewEvent(messageEventHandler),
		},
		self:           tangle.Options.Identity.ID(),
		issuingQueue:   schedulerutils.NewNodeQueue(tangle.Options.Identity.ID()),
		issue:          make(chan *Message, issueChanSize),
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

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (r *RateSetter) Setup() {
	r.tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID MessageID) {
		r.rateSetting(messageID)
	}))
}

func (r *RateSetter) unsubmit(message *Message) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.issuingQueue.Unsubmit(message)
	r.tangle.Scheduler.Unsubmit(message.ID())
}

// Submit submits a message to the local issuing queue.
func (r *RateSetter) Submit(message *Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.issuingQueue.Size()+uint(len(message.Bytes())) > MaxLocalQueueSize {
		r.Events.MessageDiscarded.Trigger(message.ID())
		return errors.Errorf("local queue exceeded: %s", message.ID())
	}
	submitted, err := r.issuingQueue.Submit(message)
	if err != nil {
		r.tangle.Events.Info.Trigger(errors.Errorf("failed to submit message %s to issuing queue: %w", message.ID().Base58(), err))
		return err
	}
	if submitted {
		r.issue <- message
	}
	return nil
}

// Shutdown shuts down the RateSetter.
func (r *RateSetter) Shutdown() {
	r.shutdownOnce.Do(func() {
		close(r.shutdownSignal)
	})
}

func (r *RateSetter) rateSetting(messageID MessageID) {
	cachedMessage := r.tangle.Storage.Message(messageID)
	var isIssuer bool
	cachedMessage.Consume(func(message *Message) {
		nodeID := identity.NewID(message.IssuerPublicKey())
		isIssuer = r.self == nodeID
	})
	if !isIssuer {
		return
	}

	if r.haltUpdate > 0 {
		r.haltUpdate--
		return
	}

	mana := r.tangle.Options.SchedulerParams.AccessManaRetrieveFunc(r.self)
	totalMana := r.tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc()
	if mana <= 0 {
		r.tangle.Events.Info.Trigger(errors.Errorf("invalid mana: %f when setting rate for message %s", mana, messageID.Base58()))
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
				issue.Reset(time.Until(lastIssueTime.Add(r.issueInterval(next.(*Message)))))
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
				issue.Reset(time.Until(lastIssueTime.Add(r.issueInterval(next.(*Message)))))
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
