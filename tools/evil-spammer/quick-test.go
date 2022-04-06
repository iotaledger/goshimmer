package main

import (
	"time"

	"github.com/iotaledger/goshimmer-tools/internal-tools/standardized-tests/tools"

	"github.com/iotaledger/goshimmer-tools/internal-tools/standardized-tests/spammers"
)

type QuickTestParams struct {
	ClientUrls            []string
	Rate                  int
	Duration              time.Duration
	TimeUnit              time.Duration
	DelayBetweenConflicts time.Duration
	VerifyLedger          bool
}

// QuickTest runs short spamming periods with stable mps
func QuickTest(params *QuickTestParams) *spammers.Wallets {
	clients := spammers.NewClientsFromURL(params.ClientUrls, "")
	counter := spammers.NewErrorCount()
	log.Info("Starting quick test")

	numOfOut := spammers.TotalOutputsNeeded(params.Duration, []int{params.Rate, params.Rate}, params.TimeUnit) // double spent will need two times fewer outputs

	log.Info("Start preparing funds")
	spamWallet, err := spammers.PrepareFunds(clients, numOfOut, 5, counter)
	if err != nil {
		log.Error(err)
		return nil
	}
	outWallets := spammers.NewWallets()

	// define spammers
	baseOptions := []spammers.Options{
		spammers.WithSpamDetails(params.Rate, params.TimeUnit, params.Duration, 0),
		spammers.WithClients(clients),
		spammers.WithErrorCounter(counter),
	}
	msgOptions := append(baseOptions,
		spammers.WithSpammingFunc(spammers.MessageSpammingFunc, false),
	)
	txOptions := append(baseOptions, []spammers.Options{
		spammers.WithOutputWallets(outWallets),
		spammers.WithSpamWallet(spamWallet),
		spammers.WithSpammingFunc(spammers.TransactionSpammingFunc, true),
	}...)
	dsOptions := append(baseOptions, []spammers.Options{
		spammers.WithOutputWallets(outWallets),
		spammers.WithSpamWallet(spamWallet),
		spammers.WithTimeDelayForDoubleSpend(params.DelayBetweenConflicts),
		spammers.WithSpammingFunc(spammers.DoubleSpendSpammingFunc, true),
	}...)

	msgSpammer := spammers.NewSpammer(msgOptions...)
	txSpammer := spammers.NewSpammer(txOptions...)
	dsSpammer := spammers.NewSpammer(dsOptions...)

	// start test
	txSpammer.Spam()
	time.Sleep(10 * time.Second)

	msgSpammer.Spam()
	awaitTipPoolSizeIsSmallEnough(clients.GetClients(1)[0], 2*time.Minute, 100)

	dsSpammer.Spam()

	log.Info(counter.GetErrorsSummary())

	if params.VerifyLedger {
		log.Infof("Ledger verification params.Starts within 10s...")
		time.Sleep(time.Second * 10)
		tools.VerifyBranches(clients)
	}

	log.Info("Quick Test finished")

	return outWallets
}
