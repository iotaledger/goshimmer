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
}

func CustomSpam(params *CustomSpamParams) {
	wallet := evilwallet.NewEvilWallet(params.ClientUrls...)
	outWallet := wallet.NewWallet(evilwallet.Reuse)

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
				SpamTransaction(wallet, outWallet, params.Rates[i], params.TimeUnit, time.Duration(params.DurationsInSec[i])*time.Second)
			}()
		case "ds":
			wg.Add(1)
			go func() {
				defer wg.Done()
				SpamDoubleSpends(wallet, outWallet, params.Rates[i], params.MsgToBeSent[i], params.TimeUnit, time.Duration(params.DurationsInSec[i])*time.Second, params.DelayBetweenConflicts)
			}()

		default:
			log.Warn("Spamming type not recognized. Try one of following: tx, ds, msg")
		}
	}

	wg.Wait()
	log.Info("Basic spamming finished!")
}

func SpamTransaction(wallet *evilwallet.EvilWallet, outWallet *evilwallet.Wallet, rate int, timeUnit, duration time.Duration) {
	scenarioTx := evilwallet.NewEvilScenario(evilwallet.SingleTransactionBatch(), false, outWallet)

	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithSpamWallet(wallet),
		evilspammer.WithEvilScenario(scenarioTx),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}

func SpamDoubleSpends(wallet *evilwallet.EvilWallet, outWallet *evilwallet.Wallet, rate, numDsToSend int, timeUnit, duration, delayBetweenConflicts time.Duration) {
	scenarioDs := evilwallet.NewEvilScenario(evilwallet.DoubleSpendBatch(numDsToSend), false, outWallet)
	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithSpamWallet(wallet),
		evilspammer.WithTimeDelayForDoubleSpend(delayBetweenConflicts),
		evilspammer.WithEvilScenario(scenarioDs),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}

func SpamMessages(wallet *evilwallet.EvilWallet, rate int, timeUnit, duration time.Duration, numMsgToSend int) {
	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithBatchesSent(numMsgToSend),
		evilspammer.WithSpamWallet(wallet),
		evilspammer.WithSpammingFunc(evilspammer.DataSpammingFunction),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}
