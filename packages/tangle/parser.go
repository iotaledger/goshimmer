package tangle

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/bytesfilter"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/typeutils"
)

const (
	// MaxReattachmentTimeMin defines the max reattachment time.
	MaxReattachmentTimeMin = 10 * time.Minute
)

// region Parser ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Parser parses messages and bytes and emits corresponding events for parsed and rejected messages.
type Parser struct {
	bytesFilters   []BytesFilter
	messageFilters []MessageFilter
	Events         *ParserEvents

	byteFiltersModified    typeutils.AtomicBool
	messageFiltersModified typeutils.AtomicBool
	bytesFiltersMutex      sync.Mutex
	messageFiltersMutex    sync.Mutex
}

// NewParser creates a new Message parser.
func NewParser() (result *Parser) {
	result = &Parser{
		bytesFilters:   make([]BytesFilter, 0),
		messageFilters: make([]MessageFilter, 0),
		Events: &ParserEvents{
			MessageParsed:   events.NewEvent(messageParsedEventHandler),
			BytesRejected:   events.NewEvent(bytesRejectedEventHandler),
			MessageRejected: events.NewEvent(messageRejectedEventHandler),
		},
	}

	// add builtin filters
	result.AddBytesFilter(NewRecentlySeenBytesFilter())
	result.AddMessageFilter(NewMessageSignatureFilter())
	result.AddMessageFilter(NewTransactionFilter())
	return
}

// Setup defines the flow of the parser.
func (p *Parser) Setup() {
	p.setupBytesFilterDataFlow()
	p.setupMessageFilterDataFlow()
}

// Parse parses the given message bytes.
func (p *Parser) Parse(messageBytes []byte, peer *peer.Peer) {
	p.bytesFilters[0].Filter(messageBytes, peer)
}

// AddBytesFilter adds the given bytes filter to the parser.
func (p *Parser) AddBytesFilter(filter BytesFilter) {
	p.bytesFiltersMutex.Lock()
	p.bytesFilters = append(p.bytesFilters, filter)
	p.bytesFiltersMutex.Unlock()
	p.byteFiltersModified.Set()
}

// AddMessageFilter adds a new message filter to the parser.
func (p *Parser) AddMessageFilter(filter MessageFilter) {
	p.messageFiltersMutex.Lock()
	p.messageFilters = append(p.messageFilters, filter)
	p.messageFiltersMutex.Unlock()
	p.messageFiltersModified.Set()
}

// sets up the byte filter data flow chain.
func (p *Parser) setupBytesFilterDataFlow() {
	if !p.byteFiltersModified.IsSet() {
		return
	}

	p.bytesFiltersMutex.Lock()
	if p.byteFiltersModified.IsSet() {
		p.byteFiltersModified.SetTo(false)

		numberOfBytesFilters := len(p.bytesFilters)
		for i := 0; i < numberOfBytesFilters; i++ {
			if i == numberOfBytesFilters-1 {
				p.bytesFilters[i].OnAccept(p.parseMessage)
			} else {
				p.bytesFilters[i].OnAccept(p.bytesFilters[i+1].Filter)
			}
			p.bytesFilters[i].OnReject(func(bytes []byte, err error, peer *peer.Peer) {
				p.Events.BytesRejected.Trigger(&BytesRejectedEvent{
					Bytes: bytes,
					Peer:  peer}, err)
			})
		}
	}
	p.bytesFiltersMutex.Unlock()
}

// sets up the message filter data flow chain.
func (p *Parser) setupMessageFilterDataFlow() {
	if !p.messageFiltersModified.IsSet() {
		return
	}

	p.messageFiltersMutex.Lock()
	if p.messageFiltersModified.IsSet() {
		p.messageFiltersModified.SetTo(false)

		numberOfMessageFilters := len(p.messageFilters)
		for i := 0; i < numberOfMessageFilters; i++ {
			if i == numberOfMessageFilters-1 {
				p.messageFilters[i].OnAccept(func(msg *Message, peer *peer.Peer) {
					p.Events.MessageParsed.Trigger(&MessageParsedEvent{
						Message: msg,
						Peer:    peer})
				})
			} else {
				p.messageFilters[i].OnAccept(p.messageFilters[i+1].Filter)
			}
			p.messageFilters[i].OnReject(func(msg *Message, err error, peer *peer.Peer) {
				p.Events.MessageRejected.Trigger(&MessageRejectedEvent{
					Message: msg,
					Peer:    peer}, err)
			})
		}
	}
	p.messageFiltersMutex.Unlock()
}

// parses the given message and emits
func (p *Parser) parseMessage(bytes []byte, peer *peer.Peer) {
	if parsedMessage, _, err := MessageFromBytes(bytes); err != nil {
		p.Events.BytesRejected.Trigger(&BytesRejectedEvent{
			Bytes: bytes,
			Peer:  peer}, err)
	} else {
		p.messageFilters[0].Filter(parsedMessage, peer)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ParserEvents /////////////////////////////////////////////////////////////////////////////////////////////////

// ParserEvents represents events happening in the Parser.
type ParserEvents struct {
	// Fired when a message was parsed.
	MessageParsed *events.Event

	// Fired when submitted bytes are rejected by a filter.
	BytesRejected *events.Event

	// Fired when a message got rejected by a filter.
	MessageRejected *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BytesRejectedEvent ///////////////////////////////////////////////////////////////////////////////////////////

// BytesRejectedEvent holds the information provided by the BytesRejected event that gets triggered when the bytes of a
// Message did not pass the parsing step.
type BytesRejectedEvent struct {
	Bytes []byte
	Peer  *peer.Peer
}

func bytesRejectedEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(*BytesRejectedEvent, error))(params[0].(*BytesRejectedEvent), params[1].(error))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageRejectedEvent /////////////////////////////////////////////////////////////////////////////////////////

// MessageRejectedEvent holds the information provided by the MessageRejected event that gets triggered when the Message
// was detected to be invalid.
type MessageRejectedEvent struct {
	Message *Message
	Peer    *peer.Peer
}

func messageRejectedEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(*MessageRejectedEvent, error))(params[0].(*MessageRejectedEvent), params[1].(error))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageParsedEvent ///////////////////////////////////////////////////////////////////////////////////////////

// MessageParsedEvent holds the information provided by the MessageParsed event that gets triggered when a message was
// fully parsed and syntactically validated.
type MessageParsedEvent struct {
	// Message contains the parsed Message.
	Message *Message

	// Peer contains the node that sent this Message to the node.
	Peer *peer.Peer
}

func messageParsedEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(*MessageParsedEvent))(params[0].(*MessageParsedEvent))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BytesFilter //////////////////////////////////////////////////////////////////////////////////////////////////

// BytesFilter filters based on byte slices and peers.
type BytesFilter interface {
	// Filter filters up on the given bytes and peer and calls the acceptance callback
	// if the input passes or the rejection callback if the input is rejected.
	Filter(bytes []byte, peer *peer.Peer)
	// OnAccept registers the given callback as the acceptance function of the filter.
	OnAccept(callback func(bytes []byte, peer *peer.Peer))
	// OnReject registers the given callback as the rejection function of the filter.
	OnReject(callback func(bytes []byte, err error, peer *peer.Peer))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageFilter ////////////////////////////////////////////////////////////////////////////////////////////////

// MessageFilter filters based on messages and peers.
type MessageFilter interface {
	// Filter filters up on the given message and peer and calls the acceptance callback
	// if the input passes or the rejection callback if the input is rejected.
	Filter(msg *Message, peer *peer.Peer)
	// OnAccept registers the given callback as the acceptance function of the filter.
	OnAccept(callback func(msg *Message, peer *peer.Peer))
	// OnAccept registers the given callback as the rejection function of the filter.
	OnReject(callback func(msg *Message, err error, peer *peer.Peer))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageSignatureFilter ///////////////////////////////////////////////////////////////////////////////////////

// MessageSignatureFilter filters messages based on whether their signatures are valid.
type MessageSignatureFilter struct {
	onAcceptCallback func(msg *Message, peer *peer.Peer)
	onRejectCallback func(msg *Message, err error, peer *peer.Peer)

	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

// NewMessageSignatureFilter creates a new message signature filter.
func NewMessageSignatureFilter() *MessageSignatureFilter {
	return &MessageSignatureFilter{}
}

// Filter filters up on the given bytes and peer and calls the acceptance callback
// if the input passes or the rejection callback if the input is rejected.
func (f *MessageSignatureFilter) Filter(msg *Message, peer *peer.Peer) {
	if msg.VerifySignature() {
		f.getAcceptCallback()(msg, peer)
		return
	}
	f.getRejectCallback()(msg, ErrInvalidSignature, peer)
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (f *MessageSignatureFilter) OnAccept(callback func(msg *Message, peer *peer.Peer)) {
	f.onAcceptCallbackMutex.Lock()
	f.onAcceptCallback = callback
	f.onAcceptCallbackMutex.Unlock()
}

// OnReject registers the given callback as the rejection function of the filter.
func (f *MessageSignatureFilter) OnReject(callback func(msg *Message, err error, peer *peer.Peer)) {
	f.onRejectCallbackMutex.Lock()
	f.onRejectCallback = callback
	f.onRejectCallbackMutex.Unlock()
}

func (f *MessageSignatureFilter) getAcceptCallback() (result func(msg *Message, peer *peer.Peer)) {
	f.onAcceptCallbackMutex.RLock()
	result = f.onAcceptCallback
	f.onAcceptCallbackMutex.RUnlock()
	return
}

func (f *MessageSignatureFilter) getRejectCallback() (result func(msg *Message, err error, peer *peer.Peer)) {
	f.onRejectCallbackMutex.RLock()
	result = f.onRejectCallback
	f.onRejectCallbackMutex.RUnlock()
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PowFilter ////////////////////////////////////////////////////////////////////////////////////////////////////

// PowFilter is a message bytes filter validating the PoW nonce.
type PowFilter struct {
	worker     *pow.Worker
	difficulty int

	mu             sync.Mutex
	acceptCallback func([]byte, *peer.Peer)
	rejectCallback func([]byte, error, *peer.Peer)
}

// NewPowFilter creates a new PoW bytes filter.
func NewPowFilter(worker *pow.Worker, difficulty int) *PowFilter {
	return &PowFilter{
		worker:     worker,
		difficulty: difficulty,
	}
}

// Filter checks whether the given bytes pass the PoW validation and calls the corresponding callback.
func (f *PowFilter) Filter(msgBytes []byte, p *peer.Peer) {
	if err := f.validate(msgBytes); err != nil {
		f.reject(msgBytes, err, p)
		return
	}
	f.accept(msgBytes, p)
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (f *PowFilter) OnAccept(callback func([]byte, *peer.Peer)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acceptCallback = callback
}

// OnReject registers the given callback as the rejection function of the filter.
func (f *PowFilter) OnReject(callback func([]byte, error, *peer.Peer)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejectCallback = callback
}

func (f *PowFilter) accept(msgBytes []byte, p *peer.Peer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.acceptCallback != nil {
		f.acceptCallback(msgBytes, p)
	}
}

func (f *PowFilter) reject(msgBytes []byte, err error, p *peer.Peer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rejectCallback != nil {
		f.rejectCallback(msgBytes, err, p)
	}
}

func (f *PowFilter) validate(msgBytes []byte) error {
	content, err := powData(msgBytes)
	if err != nil {
		return err
	}
	zeros, err := f.worker.LeadingZeros(content)
	if err != nil {
		return err
	}
	if zeros < f.difficulty {
		return fmt.Errorf("%w: leading zeros %d for difficulty %d", ErrInvalidPOWDifficultly, zeros, f.difficulty)
	}
	return nil
}

// powData returns the bytes over which PoW should be computed.
func powData(msgBytes []byte) ([]byte, error) {
	contentLength := len(msgBytes) - ed25519.SignatureSize
	if contentLength < pow.NonceBytes {
		return nil, ErrMessageTooSmall
	}
	return msgBytes[:contentLength], nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RecentlySeenBytesFilter //////////////////////////////////////////////////////////////////////////////////////

// RecentlySeenBytesFilter filters so that bytes which were recently seen don't pass the filter.
type RecentlySeenBytesFilter struct {
	bytesFilter      *bytesfilter.BytesFilter
	onAcceptCallback func(bytes []byte, peer *peer.Peer)
	onRejectCallback func(bytes []byte, err error, peer *peer.Peer)

	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

// NewRecentlySeenBytesFilter creates a new recently seen bytes filter.
func NewRecentlySeenBytesFilter() *RecentlySeenBytesFilter {
	return &RecentlySeenBytesFilter{
		bytesFilter: bytesfilter.New(100000),
	}
}

// Filter filters up on the given bytes and peer and calls the acceptance callback
// if the input passes or the rejection callback if the input is rejected.
func (r *RecentlySeenBytesFilter) Filter(bytes []byte, peer *peer.Peer) {
	if r.bytesFilter.Add(bytes) {
		r.getAcceptCallback()(bytes, peer)
		return
	}
	r.getRejectCallback()(bytes, ErrReceivedDuplicateBytes, peer)
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (r *RecentlySeenBytesFilter) OnAccept(callback func(bytes []byte, peer *peer.Peer)) {
	r.onAcceptCallbackMutex.Lock()
	r.onAcceptCallback = callback
	r.onAcceptCallbackMutex.Unlock()
}

// OnReject registers the given callback as the rejection function of the filter.
func (r *RecentlySeenBytesFilter) OnReject(callback func(bytes []byte, err error, peer *peer.Peer)) {
	r.onRejectCallbackMutex.Lock()
	r.onRejectCallback = callback
	r.onRejectCallbackMutex.Unlock()
}

func (r *RecentlySeenBytesFilter) getAcceptCallback() (result func(bytes []byte, peer *peer.Peer)) {
	r.onAcceptCallbackMutex.Lock()
	result = r.onAcceptCallback
	r.onAcceptCallbackMutex.Unlock()
	return
}

func (r *RecentlySeenBytesFilter) getRejectCallback() (result func(bytes []byte, err error, peer *peer.Peer)) {
	r.onRejectCallbackMutex.Lock()
	result = r.onRejectCallback
	r.onRejectCallbackMutex.Unlock()
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionFilter ////////////////////////////////////////////////////////////////////////////////////////////

// NewTransactionFilter creates a new transaction filter.
func NewTransactionFilter() *TransactionFilter {
	return &TransactionFilter{}
}

// TransactionFilter filters messages based on their timestamps and transaction timestamp.
type TransactionFilter struct {
	onAcceptCallback func(msg *Message, peer *peer.Peer)
	onRejectCallback func(msg *Message, err error, peer *peer.Peer)

	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

// Filter compares the timestamps between the message and it's transaction payload and calls the corresponding callback.
func (f *TransactionFilter) Filter(msg *Message, peer *peer.Peer) {
	if payload := msg.Payload(); payload.Type() == ledgerstate.TransactionType {
		transaction, _, err := ledgerstate.TransactionFromBytes(payload.Bytes())
		if err != nil {
			f.getRejectCallback()(msg, err, peer)
			return
		}
		if !isMessageAndTransactionTimestampsValid(transaction, msg) {
			f.getRejectCallback()(msg, ErrInvalidMessageAndTransactionTimestamp, peer)
			return
		}
	}
	f.getAcceptCallback()(msg, peer)
}

func isMessageAndTransactionTimestampsValid(transaction *ledgerstate.Transaction, message *Message) bool {
	transactionTimestamp := transaction.Essence().Timestamp()
	messageTimestamp := message.IssuingTime()
	return messageTimestamp.Sub(transactionTimestamp).Milliseconds() >= 0 && messageTimestamp.Sub(transactionTimestamp) <= MaxReattachmentTimeMin
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (f *TransactionFilter) OnAccept(callback func(msg *Message, peer *peer.Peer)) {
	f.onAcceptCallbackMutex.Lock()
	defer f.onAcceptCallbackMutex.Unlock()
	f.onAcceptCallback = callback
}

// OnReject registers the given callback as the rejection function of the filter.
func (f *TransactionFilter) OnReject(callback func(msg *Message, err error, peer *peer.Peer)) {
	f.onRejectCallbackMutex.Lock()
	defer f.onRejectCallbackMutex.Unlock()
	f.onRejectCallback = callback
}

func (f *TransactionFilter) getAcceptCallback() (result func(msg *Message, peer *peer.Peer)) {
	f.onAcceptCallbackMutex.RLock()
	result = f.onAcceptCallback
	f.onAcceptCallbackMutex.RUnlock()
	return
}

func (f *TransactionFilter) getRejectCallback() (result func(msg *Message, err error, peer *peer.Peer)) {
	f.onRejectCallbackMutex.RLock()
	result = f.onRejectCallback
	f.onRejectCallbackMutex.RUnlock()
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Errors ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// ErrInvalidPOWDifficultly is returned when the nonce of a message does not fulfill the PoW difficulty.
	ErrInvalidPOWDifficultly = errors.New("invalid PoW")

	// ErrMessageTooSmall is returned when the message does not contain enough data for the PoW.
	ErrMessageTooSmall = errors.New("message too small")

	// ErrInvalidSignature is returned when a message contains an invalid signature.
	ErrInvalidSignature = fmt.Errorf("invalid signature")

	// ErrReceivedDuplicateBytes is returned when duplicated bytes are rejected.
	ErrReceivedDuplicateBytes = fmt.Errorf("received duplicate bytes")

	// ErrInvalidMessageAndTransactionTimestamp is returned when the message its transaction timestamps are invalid.
	ErrInvalidMessageAndTransactionTimestamp = fmt.Errorf("invalid message and transaction timestamp")
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
