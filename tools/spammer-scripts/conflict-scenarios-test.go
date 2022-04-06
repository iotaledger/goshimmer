package main

import (
	"time"

	"github.com/iotaledger/goshimmer-tools/internal-tools/standardized-tests/spammers"
	"github.com/iotaledger/goshimmer-tools/internal-tools/standardized-tests/tools"
)

func ConflictScenarioTest1(clients *spammers.Clients) {
	log.Info("Starting quick test")
	counter := spammers.NewErrorCount()
	options := []spammers.Options{
		spammers.WithSpamDetails(10, time.Second, 0, 100),
		spammers.WithClients(clients),
		spammers.WithLogTickerInterval(time.Second * 2),
		spammers.WithSpammingFunc(spammers.ConflictSetSpammingFunc, true),
		spammers.WithErrorCounter(counter),
		spammers.WithIssueAPIMethod("SendPayload"),
	}
	spammer := spammers.NewSpammer(options...)
	spammer.Spam()

	log.Info(counter.GetErrorsSummary())
	log.Info("Test scenario 1 finished")

	time.Sleep(time.Second * 20)
	tools.VerifyBranches(clients)
}
