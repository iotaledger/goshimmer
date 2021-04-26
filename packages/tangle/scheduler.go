package tangle

import (
	"container/heap"
	"container/ring"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"golang.org/x/xerrors"
)

const (
	// MaxBufferSize is the maximum total (of all nodes) buffer size in bytes
	MaxBufferSize = 10 * 1024 * 1024
	// MaxDeficit is the maximum deficit, i.e. max bytes that can be scheduled without waiting; >= maxMessageSize
	MaxDeficit = MaxMessageSize
)

var (
	// MaxQueueWeight is the maximum mana-scaled inbox size; >= minMessageSize / minAccessMana
	MaxQueueWeight = 1024.0
	// rate is the minimum time interval between two scheduled messages, i.e. 1s / MPS
	rate = time.Second / 200
)

var (
	// ErrBufferFull is returned when the maximum buffer size is exceeded.
	ErrBufferFull = errors.New("maximum buffer size exceeded")
	// ErrInboxExceeded is returned when a node has exceeded its allowed inbox size.
	ErrInboxExceeded = errors.New("maximum mana-scaled inbox length exceeded")
	// ErrInvalidMana is returned when the mana is <= 0.
	ErrInvalidMana = errors.New("mana cannot be <= 0")
)

// AccessManaRetrieveFunc is a function type to retrieve access mana (e.g. via the mana plugin)
type AccessManaRetrieveFunc func(nodeID identity.ID) float64

// TotalAccessManaRetrieveFunc is a function type to retrieve the total access mana (e.g. via the mana plugin)
type TotalAccessManaRetrieveFunc func() float64

// TODO: make params nullable.

// SchedulerParams defines the scheduler config parameters.
type SchedulerParams struct {
	Rate                        time.Duration
	MaxQueueWeight              *float64
	AccessManaRetrieveFunc      func(identity.ID) float64
	TotalAccessManaRetrieveFunc func() float64
}

// Scheduler is a Tangle component that takes care of scheduling the messages that shall be booked.
type Scheduler struct {
	Events           *SchedulerEvents
	tangle           *Tangle
	self             identity.ID
	mu               sync.Mutex
	buffer           *BufferQueue
	deficits         map[identity.ID]float64
	onMessageSolid   *events.Closure
	onMessageInvalid *events.Closure
	shutdownSignal   chan struct{}
	shutdownOnce     sync.Once
	ticker           *time.Ticker
}

// NewScheduler returns a new Scheduler.
func NewScheduler(tangle *Tangle) *Scheduler {
	if tangle.Options.SchedulerParams.AccessManaRetrieveFunc == nil || tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc == nil {
		panic("the option AccessManaRetriever and TotalAccessManaRetriever must be defined so that AccessMana can be determined in scheduler")
	}
	if tangle.Options.SchedulerParams.MaxQueueWeight != nil {
		MaxQueueWeight = *tangle.Options.SchedulerParams.MaxQueueWeight
	}
	if tangle.Options.SchedulerParams.Rate > 0 {
		rate = tangle.Options.SchedulerParams.Rate
	}

	scheduler := &Scheduler{
		Events: &SchedulerEvents{
			MessageScheduled: events.NewEvent(MessageIDCaller),
			MessageDiscarded: events.NewEvent(MessageIDCaller),
			NodeBlacklisted:  events.NewEvent(NodeIDCaller),
		},
		self:           tangle.Options.Identity.ID(),
		tangle:         tangle,
		buffer:         NewBufferQueue(),
		deficits:       make(map[identity.ID]float64),
		shutdownSignal: make(chan struct{}),
	}
	scheduler.onMessageSolid = events.NewClosure(scheduler.onMessageSolidHandler)
	scheduler.onMessageInvalid = events.NewClosure(scheduler.onMessageInvalidHandler)
	return scheduler
}

func (s *Scheduler) onMessageSolidHandler(messageID MessageID) {
	s.SubmitAndReadyMessage(messageID)
}

func (s *Scheduler) onMessageInvalidHandler(messageID MessageID) {
	s.Unsubmit(messageID)
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	go s.mainLoop()
}

// Detach detaches the scheduler from the tangle events.
func (s *Scheduler) Detach() {
	s.tangle.Solidifier.Events.MessageSolid.Detach(s.onMessageSolid)
	s.tangle.Events.MessageInvalid.Detach(s.onMessageInvalid)
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (s *Scheduler) Setup() {
	s.tangle.Solidifier.Events.MessageSolid.Attach(s.onMessageSolid)
	s.tangle.Events.MessageInvalid.Attach(s.onMessageInvalid)

	//  TODO: wait for all messages to be scheduled here or in message layer?
	/*
		s.tangle.ConsensusManager.Events.MessageOpinionFormed.Attach(events.NewClosure(func(messageID MessageID) {
			if s.scheduledMessages.Delete(messageID) {
				s.allMessagesScheduledWG.Done()
			}
		}))
	*/
}

// SubmitAndReadyMessage submits the message to the scheduler and makes it ready when it's parents are booked.
func (s *Scheduler) SubmitAndReadyMessage(messageID MessageID) {
	err := s.Submit(messageID)
	if err != nil {
		s.tangle.Events.Error.Trigger(xerrors.Errorf("error in Scheduler Submit: %v", err))
		return
	}

	err = s.Ready(messageID)
	if err != nil {
		s.tangle.Events.Error.Trigger(xerrors.Errorf("error in Scheduler Ready: %v", err))
		return
	}
}

// SetRate sets the rate of the scheduler.
func (s *Scheduler) SetRate(rate time.Duration) {
	s.ticker = time.NewTicker(rate)
	s.tangle.Options.SchedulerParams.Rate = rate
}

// Shutdown shuts down the Scheduler.
func (s *Scheduler) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.shutdownSignal)
	})
}

// Submit submits a message to be considered by the scheduler.
// This transactions will be included in all the control metrics, but it will never be
// scheduled until Ready(messageID) has been called.
func (s *Scheduler) Submit(messageID MessageID) error {
	var err error

	if !s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		s.mu.Lock()
		defer s.mu.Unlock()

		nodeID := identity.NewID(message.IssuerPublicKey())
		// get the current access mana inside the lock
		mana := s.tangle.Options.SchedulerParams.AccessManaRetrieveFunc(nodeID)
		if mana <= 0 {
			err = ErrInvalidMana
			s.Events.MessageDiscarded.Trigger(messageID)
			return
		}

		err = s.buffer.Submit(message, mana)
		if err != nil {
			s.Events.MessageDiscarded.Trigger(messageID)
		}
		if errors.Is(err, ErrInboxExceeded) {
			s.Events.NodeBlacklisted.Trigger(nodeID)
		}
	}) {
		err = xerrors.Errorf("failed to get message '%x' from storage", messageID)
	}

	return err
}

// Unsubmit removes a message from the submitted messages.
// If that message is already marked as ready, Unsubmit has no effect.
func (s *Scheduler) Unsubmit(messageID MessageID) {
	if !s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.buffer.Unsubmit(message)
	}) {
		s.tangle.Events.Error.Trigger(xerrors.Errorf("error in Scheduler (Unsubmit): failed to get message '%x' from storage", messageID))
	}
}

// Ready marks a previously submitted message as ready to be scheduled.
// If Ready is called without a previous Submit, it has no effect.
func (s *Scheduler) Ready(messageID MessageID) error {
	var err error

	if !s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.buffer.Ready(message)
	}) {
		err = xerrors.Errorf("failed to get message '%x' from storage", messageID)
	}

	return err
}

// RemoveNode removes all messages (submitted and ready) for the given node.
func (s *Scheduler) RemoveNode(nodeID identity.ID) (err error) {
	if nodeID == s.self {
		return xerrors.Errorf("Invalid node to remove")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// TODO: is it necessary to trigger MessageDiscarded for all the removed messages.
	s.buffer.RemoveNode(nodeID)
	return
}

func (s *Scheduler) parentsBooked(messageID MessageID) (parentsBooked bool) {
	s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		parentsBooked = true
		message.ForEachParent(func(parent Parent) {
			if !parentsBooked || parent.ID == EmptyMessageID {
				return
			}

			if !s.tangle.Storage.MessageMetadata(parent.ID).Consume(func(messageMetadata *MessageMetadata) {
				parentsBooked = parentsBooked && messageMetadata.IsBooked()
			}) {
				parentsBooked = false
			}
		})
	})

	return
}

func (s *Scheduler) schedule() *Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	// no messages submitted
	if s.buffer.Size() == 0 {
		return nil
	}

	start := s.buffer.Current()
	now := time.Now()
	for q := start; ; {
		f := q.Front() // check whether the front of the queue can be scheduled
		if f != nil && s.getDeficit(q.NodeID()) >= float64(len(f.Bytes())) && !now.Before(f.IssuingTime()) {
			break
		}
		// otherwise increase the deficit
		mana := s.tangle.Options.SchedulerParams.AccessManaRetrieveFunc(q.NodeID())
		err := s.setDeficit(q.NodeID(), s.getDeficit(q.NodeID())+mana)
		if err != nil {
			s.tangle.Events.Error.Trigger(err)
			return nil
		}

		// TODO: different from spec
		q = s.buffer.Next()
		if q == nil {
			s.tangle.Events.Error.Trigger(xerrors.Errorf("error in Scheduler (schedule): failed to get next message from bufferQueue"))
			return nil
		}
		if q == start {
			return nil
		}
	}

	// will stay in buffer
	if !s.parentsBooked(s.buffer.Current().Front().ID()) {
		return nil
	}
	msg := s.buffer.PopFront()
	nodeID := identity.NewID(msg.IssuerPublicKey())
	err := s.setDeficit(nodeID, s.getDeficit(nodeID)-float64(len(msg.Bytes())))
	if err != nil {
		s.tangle.Events.Error.Trigger(err)
		return nil
	}

	return msg
}

// mainLoop periodically triggers the scheduling of ready messages.
func (s *Scheduler) mainLoop() {
	s.ticker = time.NewTicker(rate)
	defer s.ticker.Stop()

	for {
		select {
		// every rate time units
		case <-s.ticker.C:
			// TODO: do we need to pause the ticker, if there are no ready messages
			msg := s.schedule()
			if msg != nil {
				s.Events.MessageScheduled.Trigger(msg.ID())
			}

		// on close, exit the loop
		case <-s.shutdownSignal:
			return
		}
	}
}

func (s *Scheduler) getDeficit(nodeID identity.ID) float64 {
	return s.deficits[nodeID]
}

func (s *Scheduler) setDeficit(nodeID identity.ID, deficit float64) (err error) {
	if deficit < 0 {
		return xerrors.Errorf("error in Scheduler (setDeficit): deficit is less than 0")
	}
	s.deficits[nodeID] = math.Min(deficit, MaxDeficit)

	return
}

// NodeQueueSize returns the size of the nodeIDs queue.
func (s *Scheduler) NodeQueueSize(nodeID identity.ID) uint {
	return s.buffer.NodeQueue(nodeID).Size()
}

// region NodeQueue /////////////////////////////////////////////////////////////////////////////////////////////

// NodeQueue keeps the submitted messages of a node
type NodeQueue struct {
	nodeID    identity.ID
	submitted map[MessageID]*Message
	inbox     *MessageHeap
	size      uint
}

// NewNodeQueue returns a new NodeQueue
func NewNodeQueue(nodeID identity.ID) *NodeQueue {
	return &NodeQueue{
		nodeID:    nodeID,
		submitted: make(map[MessageID]*Message),
		inbox:     new(MessageHeap),
		size:      0,
	}
}

// IsInactive returns true when the node is inactive, i.e. there are no messages in the queue.
func (q *NodeQueue) IsInactive() bool {
	return q.Size() == 0
}

// Size returns the total size of the messages in the queue.
func (q *NodeQueue) Size() uint {
	if q == nil {
		return 0
	}
	return q.size
}

// NodeID returns the ID of the node belonging to the queue.
func (q *NodeQueue) NodeID() identity.ID {
	return q.nodeID
}

// Submit submits a message for the queue.
func (q *NodeQueue) Submit(msg *Message) (bool, error) {
	msgNodeID := identity.NewID(msg.IssuerPublicKey())
	if q.nodeID != msgNodeID {
		return false, xerrors.Errorf("Queue node ID(%x) and issuer ID(%x) doesn't match.", q.nodeID, msgNodeID)
	}
	if _, submitted := q.submitted[msg.ID()]; submitted {
		return false, nil
	}

	q.submitted[msg.ID()] = msg
	q.size += uint(len(msg.Bytes()))
	return true, nil
}

// Unsubmit removes a previously submitted message from the queue.
func (q *NodeQueue) Unsubmit(msg *Message) bool {
	if _, submitted := q.submitted[msg.ID()]; !submitted {
		return false
	}

	delete(q.submitted, msg.ID())
	q.size -= uint(len(msg.Bytes()))
	return true
}

// Ready marks a previously submitted message as ready to be scheduled.
func (q *NodeQueue) Ready(msg *Message) bool {
	if _, submitted := q.submitted[msg.ID()]; !submitted {
		return false
	}

	delete(q.submitted, msg.ID())
	heap.Push(q.inbox, msg)
	return true
}

// Front returns the first ready message in the queue.
func (q *NodeQueue) Front() *Message {
	if q == nil || q.inbox.Len() == 0 {
		return nil
	}
	return (*q.inbox)[0]
}

// PopFront removes the first ready message from the queue.
func (q *NodeQueue) PopFront() *Message {
	msg := heap.Pop(q.inbox).(*Message)
	q.size -= uint(len(msg.Bytes()))
	return msg
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BufferQueue /////////////////////////////////////////////////////////////////////////////////////////////

// BufferQueue represents a buffer of NodeQueue
type BufferQueue struct {
	activeNode map[identity.ID]*ring.Ring
	ring       *ring.Ring
	size       uint
}

// NewBufferQueue returns a new BufferQueue
func NewBufferQueue() *BufferQueue {
	return &BufferQueue{
		activeNode: make(map[identity.ID]*ring.Ring),
		ring:       nil,
		size:       0,
	}
}

// NumActiveNodes returns the number of active nodes in b.
func (b *BufferQueue) NumActiveNodes() int {
	return len(b.activeNode)
}

// Size returns the total size (in bytes) of all messages in b.
func (b *BufferQueue) Size() uint {
	return b.size
}

// NodeQueue returns the queue for the corresponding node.
func (b *BufferQueue) NodeQueue(nodeID identity.ID) *NodeQueue {
	element, ok := b.activeNode[nodeID]
	if !ok {
		return nil
	}
	return element.Value.(*NodeQueue)
}

// Submit submits a message.
func (b *BufferQueue) Submit(msg *Message, rep float64) error {
	if b.size+uint(len(msg.Bytes())) > MaxBufferSize {
		return ErrBufferFull
	}

	nodeID := identity.NewID(msg.IssuerPublicKey())
	element, ok := b.activeNode[nodeID]
	if !ok {
		element = b.ringInsert(NewNodeQueue(nodeID))
		b.activeNode[nodeID] = element
	}

	nodeQueue := element.Value.(*NodeQueue)
	if float64(nodeQueue.Size()+uint(len(msg.Bytes())))/rep > MaxQueueWeight {
		return ErrInboxExceeded
	}

	submitted, err := nodeQueue.Submit(msg)
	if err != nil {
		return err
	}
	if !submitted {
		return xerrors.Errorf("error in BufferQueue (Submit): message has already been submitted %x", msg.ID)
	}

	b.size += uint(len(msg.Bytes()))
	return nil
}

// Unsubmit removes a message from the submitted messages.
// If that message is already marked as ready, Unsubmit has no effect.
func (b *BufferQueue) Unsubmit(msg *Message) bool {
	nodeID := identity.NewID(msg.IssuerPublicKey())
	element, ok := b.activeNode[nodeID]
	if !ok {
		return false
	}

	nodeQueue := element.Value.(*NodeQueue)
	if !nodeQueue.Unsubmit(msg) {
		return false
	}

	b.size -= uint(len(msg.Bytes()))
	if nodeQueue.IsInactive() {
		b.ringRemove(element)
		delete(b.activeNode, nodeID)
	}
	return true
}

// Ready marks a previously submitted message as ready to be scheduled.
func (b *BufferQueue) Ready(msg *Message) bool {
	element, ok := b.activeNode[identity.NewID(msg.IssuerPublicKey())]
	if !ok {
		return false
	}

	nodeQueue := element.Value.(*NodeQueue)
	return nodeQueue.Ready(msg)
}

// RemoveNode removes all messages (submitted and ready) for the given node.
func (b *BufferQueue) RemoveNode(nodeID identity.ID) {
	element, ok := b.activeNode[nodeID]
	if !ok {
		return
	}

	nodeQueue := element.Value.(*NodeQueue)
	b.size -= nodeQueue.Size()

	b.ringRemove(element)
	delete(b.activeNode, nodeID)
}

// Next returns the next NodeQueue in round robin order.
func (b *BufferQueue) Next() *NodeQueue {
	if b.ring != nil {
		b.ring = b.ring.Next()
		return b.ring.Value.(*NodeQueue)
	}
	return nil
}

// Current returns the current NodeQueue in round robin order.
func (b *BufferQueue) Current() *NodeQueue {
	if b.ring == nil {
		return nil
	}
	return b.ring.Value.(*NodeQueue)
}

// PopFront removes the first ready message from the queue of the current node.
func (b *BufferQueue) PopFront() *Message {
	q := b.Current()
	msg := q.PopFront()
	if q.IsInactive() {
		b.ringRemove(b.ring)
		delete(b.activeNode, identity.NewID(msg.IssuerPublicKey()))
	}

	b.size -= uint(len(msg.Bytes()))
	return msg
}

func (b *BufferQueue) ringRemove(r *ring.Ring) {
	n := b.ring.Next()
	if r == b.ring {
		if n == b.ring {
			b.ring = nil
			return
		}
		b.ring = n
	}
	r.Prev().Link(n)
}

func (b *BufferQueue) ringInsert(v interface{}) *ring.Ring {
	p := ring.New(1)
	p.Value = v
	if b.ring == nil {
		b.ring = p
		return p
	}
	return p.Link(b.ring)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SchedulerEvents represents events happening in the Scheduler.
type SchedulerEvents struct {
	// MessageScheduled is triggered when a message is ready to be scheduled.
	MessageScheduled *events.Event
	MessageDiscarded *events.Event
	NodeBlacklisted  *events.Event
}

// NodeIDCaller is the caller function for events that hand over a NodeID.
func NodeIDCaller(handler interface{}, params ...interface{}) {
	handler.(func(identity.ID))(params[0].(identity.ID))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageHeap /////////////////////////////////////////////////////////////////////////////////////////////

// MessageHeap holds a heap of messages with respect to their IssuingTime.
type MessageHeap []*Message

// Len is the number of elements in the collection.
func (h MessageHeap) Len() int {
	return len(h)
}

// Less reports whether the element with index i must sort before the element with index j.
func (h MessageHeap) Less(i, j int) bool {
	return h[i].IssuingTime().Before(h[j].IssuingTime())
}

// Swap swaps the elements with indexes i and j.
func (h MessageHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push adds x as element with index Len().
// It panics if x is not types.Message.
func (h *MessageHeap) Push(x interface{}) {
	*h = append(*h, x.(*Message))
}

// Pop removes and returns element with index Len() - 1.
func (h *MessageHeap) Pop() interface{} {
	tmp := *h
	n := len(tmp)
	x := tmp[n-1]
	tmp[n-1] = nil
	*h = tmp[:n-1]
	return x
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
