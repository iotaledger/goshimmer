package messageparser

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messageparser/builtinfilters"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/typeutils"
)

// MessageParser parses messages and bytes and emits corresponding events for parsed and rejected messages.
type MessageParser struct {
	bytesFilters   []BytesFilter
	messageFilters []MessageFilter
	Events         Events

	byteFiltersModified    typeutils.AtomicBool
	messageFiltersModified typeutils.AtomicBool
	bytesFiltersMutex      sync.Mutex
	messageFiltersMutex    sync.Mutex
}

// New creates a new message parser.
func New() (result *MessageParser) {
	result = &MessageParser{
		bytesFilters:   make([]BytesFilter, 0),
		messageFilters: make([]MessageFilter, 0),
		Events: Events{
			MessageParsed: events.NewEvent(func(handler interface{}, params ...interface{}) {
				handler.(func(*message.Message, *peer.Peer))(params[0].(*message.Message), params[1].(*peer.Peer))
			}),
			BytesRejected: events.NewEvent(func(handler interface{}, params ...interface{}) {
				handler.(func([]byte, error, *peer.Peer))(params[0].([]byte), params[1].(error), params[2].(*peer.Peer))
			}),
			MessageRejected: events.NewEvent(func(handler interface{}, params ...interface{}) {
				handler.(func(*message.Message, error, *peer.Peer))(params[0].(*message.Message), params[1].(error), params[2].(*peer.Peer))
			}),
		},
	}

	// add builtin filters
	result.AddBytesFilter(builtinfilters.NewRecentlySeenBytesFilter())
	result.AddMessageFilter(builtinfilters.NewMessageSignatureFilter())
	return
}

// Parse parses the given message bytes.
func (messageParser *MessageParser) Parse(messageBytes []byte, peer *peer.Peer) {
	messageParser.setupBytesFilterDataFlow()
	messageParser.setupMessageFilterDataFlow()
	messageParser.bytesFilters[0].Filter(messageBytes, peer)
}

// AddBytesFilter adds the given bytes filter to the parser.
func (messageParser *MessageParser) AddBytesFilter(filter BytesFilter) {
	messageParser.bytesFiltersMutex.Lock()
	messageParser.bytesFilters = append(messageParser.bytesFilters, filter)
	messageParser.bytesFiltersMutex.Unlock()
	messageParser.byteFiltersModified.Set()
}

// AddMessageFilter adds a new message filter to the parser.
func (messageParser *MessageParser) AddMessageFilter(filter MessageFilter) {
	messageParser.messageFiltersMutex.Lock()
	messageParser.messageFilters = append(messageParser.messageFilters, filter)
	messageParser.messageFiltersMutex.Unlock()
	messageParser.messageFiltersModified.Set()
}

// Shutdown shut downs the message parser and its corresponding registered filters.
func (messageParser *MessageParser) Shutdown() {
	messageParser.bytesFiltersMutex.Lock()
	for _, bytesFilter := range messageParser.bytesFilters {
		bytesFilter.Shutdown()
	}
	messageParser.bytesFiltersMutex.Unlock()

	messageParser.messageFiltersMutex.Lock()
	for _, messageFilter := range messageParser.messageFilters {
		messageFilter.Shutdown()
	}
	messageParser.messageFiltersMutex.Unlock()
}

// sets up the byte filter data flow chain.
func (messageParser *MessageParser) setupBytesFilterDataFlow() {
	if !messageParser.byteFiltersModified.IsSet() {
		return
	}

	messageParser.bytesFiltersMutex.Lock()
	if messageParser.byteFiltersModified.IsSet() {
		messageParser.byteFiltersModified.SetTo(false)

		numberOfBytesFilters := len(messageParser.bytesFilters)
		for i := 0; i < numberOfBytesFilters; i++ {
			if i == numberOfBytesFilters-1 {
				messageParser.bytesFilters[i].OnAccept(messageParser.parseMessage)
			} else {
				messageParser.bytesFilters[i].OnAccept(messageParser.bytesFilters[i+1].Filter)
			}
			messageParser.bytesFilters[i].OnReject(func(bytes []byte, err error, peer *peer.Peer) {
				messageParser.Events.BytesRejected.Trigger(bytes, err, peer)
			})
		}
	}
	messageParser.bytesFiltersMutex.Unlock()
}

// sets up the message filter data flow chain.
func (messageParser *MessageParser) setupMessageFilterDataFlow() {
	if !messageParser.messageFiltersModified.IsSet() {
		return
	}

	messageParser.messageFiltersMutex.Lock()
	if messageParser.messageFiltersModified.IsSet() {
		messageParser.messageFiltersModified.SetTo(false)

		numberOfMessageFilters := len(messageParser.messageFilters)
		for i := 0; i < numberOfMessageFilters; i++ {
			if i == numberOfMessageFilters-1 {
				messageParser.messageFilters[i].OnAccept(func(msg *message.Message, peer *peer.Peer) {
					messageParser.Events.MessageParsed.Trigger(msg, peer)
				})
			} else {
				messageParser.messageFilters[i].OnAccept(messageParser.messageFilters[i+1].Filter)
			}
			messageParser.messageFilters[i].OnReject(func(msg *message.Message, err error, peer *peer.Peer) {
				messageParser.Events.MessageRejected.Trigger(msg, err, peer)
			})
		}
	}
	messageParser.messageFiltersMutex.Unlock()
}

// parses the given message and emits
func (messageParser *MessageParser) parseMessage(bytes []byte, peer *peer.Peer) {
	if parsedMessage, err, _ := message.FromBytes(bytes); err != nil {
		messageParser.Events.BytesRejected.Trigger(bytes, err, peer)
	} else {
		messageParser.messageFilters[0].Filter(parsedMessage, peer)
	}
}
