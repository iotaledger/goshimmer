package transactionparser

import "github.com/iotaledger/hive.go/events"

type transactionParserEvents struct {
	BytesRejected       *events.Event
	TransactionParsed   *events.Event
	TransactionRejected *events.Event
}
