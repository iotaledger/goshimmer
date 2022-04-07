package main

import (
	"github.com/iotaledger/goshimmer/client/evilspammer"
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"time"
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
func QuickTest(params *QuickTestParams) {
	evilWallet := evilwallet.NewEvilWallet(params.ClientUrls...)
	counter := evilspammer.NewErrorCount()
	log.Info("Starting quick test")

	nWallets := 2 * evilspammer.BigWalletsNeeded(params.Rate, params.TimeUnit, params.Duration)

	log.Info("Start preparing funds")
	evilWallet.RequestFreshBigFaucetWallets(nWallets)

	// define spammers
	baseOptions := []evilspammer.Options{
		evilspammer.WithSpamRate(params.Rate, params.TimeUnit),
		evilspammer.WithSpamDuration(params.Duration),
		evilspammer.WithErrorCounter(counter),
		evilspammer.WithEvilWallet(evilWallet),
	}
	msgOptions := append(baseOptions,
		evilspammer.WithSpammingFunc(evilspammer.DataSpammingFunction),
	)

	dsScenario := evilwallet.NewEvilScenario(
		evilwallet.WithScenarioCustomConflicts(evilwallet.DoubleSpendBatch(2)),
	)

	dsOptions := append(baseOptions,
		evilspammer.WithEvilScenario(dsScenario),
	)

	msgSpammer := evilspammer.NewSpammer(msgOptions...)
	txSpammer := evilspammer.NewSpammer(baseOptions...)
	dsSpammer := evilspammer.NewSpammer(dsOptions...)

	// start test
	txSpammer.Spam()
	time.Sleep(5 * time.Second)

	msgSpammer.Spam()
	time.Sleep(5 * time.Second)

	dsSpammer.Spam()

	log.Info(counter.GetErrorsSummary())
	log.Info("Quick Test finished")
}
