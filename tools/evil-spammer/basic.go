package main

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/client/evilspammer"
	"github.com/iotaledger/goshimmer/client/evilwallet"
)

type CustomSpamParams struct {
	ClientUrls            []string
	SpamTypes             []string
	Rates                 []int
	DurationsInSec        []int
	MsgToBeSent           []int
	TimeUnit              time.Duration
	DelayBetweenConflicts time.Duration
	NSpend                int
	scenario              evilwallet.EvilBatch
}

func CustomSpam(params *CustomSpamParams) {
	wallet := evilwallet.NewEvilWallet(params.ClientUrls...)
	wg := sync.WaitGroup{}

	fundsNeeded := false
	for _, spamType := range params.SpamTypes {
		if spamType == "ds" || spamType == "tx" {
			fundsNeeded = true
		}
	}
	if fundsNeeded {
		err := wallet.RequestFreshBigFaucetWallet()
		if err != nil {
			return
		}
	}

	for i, spamType := range params.SpamTypes {
		log.Infof("Start spamming with rate: %d, time unit: %s, and spamming type: %s.", params.Rates[i], params.TimeUnit.String(), spamType)

		switch spamType {
		case "msg":
			wg.Add(1)
			go func() {
				defer wg.Done()
				SpamMessages(wallet, params.Rates[i], params.TimeUnit, time.Duration(params.DurationsInSec[i])*time.Second, params.MsgToBeSent[i])
			}()
		case "tx":
			wg.Add(1)
			go func() {
				defer wg.Done()
				SpamTransaction(wallet, params.Rates[i], params.TimeUnit, time.Duration(params.DurationsInSec[i])*time.Second)
			}()
		case "ds":
			wg.Add(1)
			go func() {
				defer wg.Done()
				SpamDoubleSpends(wallet, params.Rates[i], params.MsgToBeSent[i], params.TimeUnit, time.Duration(params.DurationsInSec[i])*time.Second, params.DelayBetweenConflicts)
			}()
		case "nds":
			wg.Add(1)
			go func() {
				defer wg.Done()
				SpamNDoubleSpends(wallet, params.Rates[i], params.NSpend, params.TimeUnit, time.Duration(params.DurationsInSec[i])*time.Second, params.DelayBetweenConflicts)
			}()
		case "nestedConflicts":
			wg.Add(1)
			go func() {
				defer wg.Done()
				SpamNestedConflicts(wallet, params.Rates[i], params.TimeUnit, time.Duration(params.DurationsInSec[i])*time.Second, evilwallet.Scenario1())
			}()

		default:
			log.Warn("Spamming type not recognized. Try one of following: tx, ds, msg")
		}
	}

	wg.Wait()
	log.Info("Basic spamming finished!")
}

func SpamTransaction(wallet *evilwallet.EvilWallet, rate int, timeUnit, duration time.Duration) {
	scenarioTx := evilwallet.NewEvilScenario(
		evilwallet.WithScenarioCustomConflicts(evilwallet.SingleTransactionBatch()),
	)

	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithEvilWallet(wallet),
		evilspammer.WithEvilScenario(scenarioTx),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}

func SpamDoubleSpends(wallet *evilwallet.EvilWallet, rate, numDsToSend int, timeUnit, duration, delayBetweenConflicts time.Duration) {
	scenarioDs := evilwallet.NewEvilScenario(
		evilwallet.WithScenarioCustomConflicts(evilwallet.DoubleSpendBatch(numDsToSend)),
	)
	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithEvilWallet(wallet),
		evilspammer.WithTimeDelayForDoubleSpend(delayBetweenConflicts),
		evilspammer.WithEvilScenario(scenarioDs),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}

func SpamNDoubleSpends(wallet *evilwallet.EvilWallet, rate, nSpend int, timeUnit, duration, delayBetweenConflicts time.Duration) {
	scenario := evilwallet.NewEvilScenario(
		evilwallet.WithScenarioCustomConflicts(evilwallet.NSpendBatch(nSpend)),
	)
	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithEvilWallet(wallet),
		evilspammer.WithTimeDelayForDoubleSpend(delayBetweenConflicts),
		evilspammer.WithEvilScenario(scenario),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}

func SpamNestedConflicts(wallet *evilwallet.EvilWallet, rate int, timeUnit, duration time.Duration, conflictBatch evilwallet.EvilBatch) {
	scenario := evilwallet.NewEvilScenario(
		evilwallet.WithScenarioCustomConflicts(conflictBatch),
	)
	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithEvilWallet(wallet),
		evilspammer.WithEvilScenario(scenario),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}

func SpamMessages(wallet *evilwallet.EvilWallet, rate int, timeUnit, duration time.Duration, numMsgToSend int) {
	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithBatchesSent(numMsgToSend),
		evilspammer.WithEvilWallet(wallet),
		evilspammer.WithSpammingFunc(evilspammer.DataSpammingFunction),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}
