package tangle

import (
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/typeutils"
)

// MessageParser parses messages and bytes and emits corresponding events for parsed and rejected messages.
type MessageParser struct {
	bytesFilters   []BytesFilter
	messageFilters []MessageFilter
	Events         *MessageParserEvents

	byteFiltersModified    typeutils.AtomicBool
	messageFiltersModified typeutils.AtomicBool
	bytesFiltersMutex      sync.Mutex
	messageFiltersMutex    sync.Mutex
}

// NewMessageParser creates a new message parser.
func NewMessageParser() (result *MessageParser) {
	result = &MessageParser{
		bytesFilters:   make([]BytesFilter, 0),
		messageFilters: make([]MessageFilter, 0),
		Events:         newMessageParserEvents(),
	}

	// add builtin filters
	result.AddBytesFilter(NewRecentlySeenBytesFilter())
	result.AddMessageFilter(NewMessageSignatureFilter())
	return
}

// Parse parses the given message bytes.
func (p *MessageParser) Parse(messageBytes []byte, peer *peer.Peer) {
	p.setupBytesFilterDataFlow()
	p.setupMessageFilterDataFlow()
	p.bytesFilters[0].Filter(messageBytes, peer)
}

// AddBytesFilter adds the given bytes filter to the parser.
func (p *MessageParser) AddBytesFilter(filter BytesFilter) {
	p.bytesFiltersMutex.Lock()
	p.bytesFilters = append(p.bytesFilters, filter)
	p.bytesFiltersMutex.Unlock()
	p.byteFiltersModified.Set()
}

// AddMessageFilter adds a new message filter to the parser.
func (p *MessageParser) AddMessageFilter(filter MessageFilter) {
	p.messageFiltersMutex.Lock()
	p.messageFilters = append(p.messageFilters, filter)
	p.messageFiltersMutex.Unlock()
	p.messageFiltersModified.Set()
}

// sets up the byte filter data flow chain.
func (p *MessageParser) setupBytesFilterDataFlow() {
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
func (p *MessageParser) setupMessageFilterDataFlow() {
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
func (p *MessageParser) parseMessage(bytes []byte, peer *peer.Peer) {
	if parsedMessage, _, err := MessageFromBytes(bytes); err != nil {
		p.Events.BytesRejected.Trigger(&BytesRejectedEvent{
			Bytes: bytes,
			Peer:  peer}, err)
	} else {
		p.messageFilters[0].Filter(parsedMessage, peer)
	}
}
