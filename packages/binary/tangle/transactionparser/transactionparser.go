package transactionparser

import (
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/transactionparser/builtinfilters"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/typeutils"
)

type TransactionParser struct {
	bytesFilters       []BytesFilter
	transactionFilters []TransactionFilter
	Events             transactionParserEvents

	byteFiltersModified        typeutils.AtomicBool
	transactionFiltersModified typeutils.AtomicBool
	bytesFiltersMutex          sync.Mutex
	transactionFiltersMutex    sync.Mutex
}

func New() (result *TransactionParser) {
	result = &TransactionParser{
		bytesFilters:       make([]BytesFilter, 0),
		transactionFilters: make([]TransactionFilter, 0),

		Events: transactionParserEvents{
			BytesRejected: events.NewEvent(func(handler interface{}, params ...interface{}) {
				handler.(func([]byte, error, *peer.Peer))(params[0].([]byte), params[1].(error), params[2].(*peer.Peer))
			}),
			TransactionParsed: events.NewEvent(func(handler interface{}, params ...interface{}) {
				handler.(func(*message.Transaction, *peer.Peer))(params[0].(*message.Transaction), params[1].(*peer.Peer))
			}),
			TransactionRejected: events.NewEvent(func(handler interface{}, params ...interface{}) {
				handler.(func(*message.Transaction, error, *peer.Peer))(params[0].(*message.Transaction), params[1].(error), params[2].(*peer.Peer))
			}),
		},
	}

	// add builtin filters
	result.AddBytesFilter(builtinfilters.NewRecentlySeenBytesFilter())
	result.AddTransactionsFilter(builtinfilters.NewTransactionSignatureFilter())

	return
}

func (transactionParser *TransactionParser) Parse(transactionBytes []byte, peer *peer.Peer) {
	transactionParser.setupBytesFilterDataFlow()
	transactionParser.setupTransactionsFilterDataFlow()

	transactionParser.bytesFilters[0].Filter(transactionBytes, peer)
}

func (transactionParser *TransactionParser) AddBytesFilter(filter BytesFilter) {
	transactionParser.bytesFiltersMutex.Lock()
	transactionParser.bytesFilters = append(transactionParser.bytesFilters, filter)
	transactionParser.bytesFiltersMutex.Unlock()

	transactionParser.byteFiltersModified.Set()
}

func (transactionParser *TransactionParser) AddTransactionsFilter(filter TransactionFilter) {
	transactionParser.transactionFiltersMutex.Lock()
	transactionParser.transactionFilters = append(transactionParser.transactionFilters, filter)
	transactionParser.transactionFiltersMutex.Unlock()

	transactionParser.transactionFiltersModified.Set()
}

func (transactionParser *TransactionParser) Shutdown() {
	transactionParser.bytesFiltersMutex.Lock()
	for _, bytesFilter := range transactionParser.bytesFilters {
		bytesFilter.Shutdown()
	}
	transactionParser.bytesFiltersMutex.Unlock()

	transactionParser.transactionFiltersMutex.Lock()
	for _, transactionFilter := range transactionParser.transactionFilters {
		transactionFilter.Shutdown()
	}
	transactionParser.transactionFiltersMutex.Unlock()
}

func (transactionParser *TransactionParser) setupBytesFilterDataFlow() {
	if !transactionParser.byteFiltersModified.IsSet() {
		return
	}

	transactionParser.bytesFiltersMutex.Lock()
	if transactionParser.byteFiltersModified.IsSet() {
		transactionParser.byteFiltersModified.SetTo(false)

		numberOfBytesFilters := len(transactionParser.bytesFilters)
		for i := 0; i < numberOfBytesFilters; i++ {
			if i == numberOfBytesFilters-1 {
				transactionParser.bytesFilters[i].OnAccept(transactionParser.parseTransaction)
			} else {
				transactionParser.bytesFilters[i].OnAccept(transactionParser.bytesFilters[i+1].Filter)
			}
			transactionParser.bytesFilters[i].OnReject(func(bytes []byte, err error, peer *peer.Peer) {
				transactionParser.Events.BytesRejected.Trigger(bytes, err, peer)
			})
		}
	}
	transactionParser.bytesFiltersMutex.Unlock()
}

func (transactionParser *TransactionParser) setupTransactionsFilterDataFlow() {
	if !transactionParser.transactionFiltersModified.IsSet() {
		return
	}

	transactionParser.transactionFiltersMutex.Lock()
	if transactionParser.transactionFiltersModified.IsSet() {
		transactionParser.transactionFiltersModified.SetTo(false)

		numberOfTransactionFilters := len(transactionParser.transactionFilters)
		for i := 0; i < numberOfTransactionFilters; i++ {
			if i == numberOfTransactionFilters-1 {
				transactionParser.transactionFilters[i].OnAccept(func(tx *message.Transaction, peer *peer.Peer) {
					transactionParser.Events.TransactionParsed.Trigger(tx, peer)
				})
			} else {
				transactionParser.transactionFilters[i].OnAccept(transactionParser.transactionFilters[i+1].Filter)
			}
			transactionParser.transactionFilters[i].OnReject(func(tx *message.Transaction, err error, peer *peer.Peer) {
				transactionParser.Events.TransactionRejected.Trigger(tx, err, peer)
			})
		}
	}
	transactionParser.transactionFiltersMutex.Unlock()
}

func (transactionParser *TransactionParser) parseTransaction(bytes []byte, peer *peer.Peer) {
	if parsedTransaction, err, _ := message.FromBytes(bytes); err != nil {
		transactionParser.Events.BytesRejected.Trigger(bytes, err, peer)
	} else {
		transactionParser.transactionFilters[0].Filter(parsedTransaction, peer)
	}
}
