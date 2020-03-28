package spammer

import (
	"sync/atomic"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tipselector"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/transactionparser"
)

type Spammer struct {
	transactionParser *transactionparser.TransactionParser
	tipSelector       *tipselector.TipSelector

	processId      int64
	shutdownSignal chan types.Empty
}

func New(transactionParser *transactionparser.TransactionParser, tipSelector *tipselector.TipSelector) *Spammer {
	return &Spammer{
		shutdownSignal:    make(chan types.Empty),
		transactionParser: transactionParser,
		tipSelector:       tipSelector,
	}
}

func (spammer *Spammer) Start(tps int) {
	go spammer.run(tps, atomic.AddInt64(&spammer.processId, 1))
}

func (spammer *Spammer) Burst(transactions int) {
	go spammer.sendBurst(transactions, atomic.AddInt64(&spammer.processId, 1))
}

func (spammer *Spammer) Shutdown() {
	atomic.AddInt64(&spammer.processId, 1)
}

func (spammer *Spammer) run(tps int, processId int64) {
	// TODO: this should be the local peer's identity
	spammingIdentity := identity.GenerateLocalIdentity()
	currentSentCounter := 0
	start := time.Now()

	for {
		if atomic.LoadInt64(&spammer.processId) != processId {
			return
		}

		// TODO: use transaction factory
		trunkTransactionId, branchTransactionId := spammer.tipSelector.GetTips()

		msg := message.New(
			trunkTransactionId,
			branchTransactionId,
			spammingIdentity.PublicKey(),
			time.Now(),
			0,
			payload.NewData([]byte("SPAM")),
			spammingIdentity,
		)
		spammer.transactionParser.Parse(msg.Bytes(), nil)

		currentSentCounter++

		// rate limit to the specified TPS
		if currentSentCounter >= tps {
			duration := time.Since(start)
			if duration < time.Second {
				time.Sleep(time.Second - duration)
			}

			start = time.Now()
			currentSentCounter = 0
		}
	}
}

func (spammer *Spammer) sendBurst(transactions int, processId int64) {
	// TODO: this should be the local peer's identity
	spammingIdentity := identity.GenerateLocalIdentity()

	previousTransactionId := message.EmptyId

	burstBuffer := make([][]byte, transactions)
	for i := 0; i < transactions; i++ {
		if atomic.LoadInt64(&spammer.processId) != processId {
			return
		}

		// TODO: use transaction factory
		spamTransaction := message.New(
			previousTransactionId,
			previousTransactionId,
			spammingIdentity.PublicKey(),
			time.Now(),
			0,
			payload.NewData([]byte("SPAM")),
			spammingIdentity,
		)

		previousTransactionId = spamTransaction.GetId()
		burstBuffer[i] = spamTransaction.Bytes()
	}

	for i := 0; i < transactions; i++ {
		if atomic.LoadInt64(&spammer.processId) != processId {
			return
		}

		spammer.transactionParser.Parse(burstBuffer[i], nil)
	}
}
