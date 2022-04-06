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
				SpamMessages(wallet, params.Rates[i], params.TimeUnit, time.Duration(params.DurationsInSec[i])*time.Second, params.MsgToBeSent[i], counter)
			}()
		case "tx":
			wg.Add(1)
			go func() {
				defer wg.Done()
				SpamTransaction(wallet, outWallet, params.Rates[i], params.MsgToBeSent[i], params.TimeUnit, time.Duration(params.DurationsInSec[i])*time.Second, counter)
			}()
		case "ds":
			wg.Add(1)
			go func() {
				defer wg.Done()
				SpamDoubleSpends(wallet, outWallet, params.Rates[i], params.MsgToBeSent[i], params.TimeUnit, time.Duration(params.DurationsInSec[i])*time.Second, params.DelayBetweenConflicts, counter)
			}()
		default:
			log.Warn("Spamming type not recognized. Try one of following: tx, ds, msg")
		}
	}

	wg.Wait()
	log.Info("Basic spamming finished!")
}

// TODO: refactor this with evilspammer
func prepareFundsForCustomSpam(clients *evilspammer.Clients, params *CustomSpamParams, counter *evilspammer.ErrorCounter) *evilspammer.Wallets {
	numOutputs := 0

	if len(params.MsgToBeSent) > 0 {
		for _, num := range params.MsgToBeSent {
			numOutputs += num
		}
	} else {
		for i := range params.DurationsInSec {
			numOutputs += int(time.Duration(params.DurationsInSec[i])*time.Second/params.TimeUnit) * params.Rates[i]
		}
	}
	spamWallets, err := evilspammer.PrepareFunds(clients, numOutputs, 5, counter)
	if err != nil {
		log.Error(err)
		return nil
	}
	return spamWallets
}

func SpamTransaction(wallet *evilwallet.EvilWallet, outWallet *evilwallet.Wallet, rate int, timeUnit, duration time.Duration, counter *evilspammer.ErrorCounter) {
	scenarioTx := evilwallet.NewEvilScenario(evilwallet.SingleTransactionBatch(), false, outWallet)

	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithSpamWallet(wallet),
		evilspammer.WithErrorCounter(counter),
		evilspammer.WithEvilScenario(scenarioTx),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}

func SpamDoubleSpends(wallet *evilwallet.EvilWallet, outWallet *evilwallet.Wallet, rate, numDsToSend int, timeUnit, duration, delayBetweenConflicts time.Duration, counter *evilspammer.ErrorCounter) {
	scenarioDs := evilwallet.NewEvilScenario(evilwallet.DoubleSpendBatch(numDsToSend), false, outWallet)
	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithSpamWallet(wallet),
		evilspammer.WithTimeDelayForDoubleSpend(delayBetweenConflicts),
		evilspammer.WithErrorCounter(counter),
		evilspammer.WithEvilScenario(scenarioDs),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}

func SpamMessages(wallet *evilwallet.EvilWallet, rate int, timeUnit, duration time.Duration, numMsgToSend int, counter *evilspammer.ErrorCounter) {
	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithBatchesSent(numMsgToSend),
		evilspammer.WithSpamWallet(wallet),
		evilspammer.WithErrorCounter(counter),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}
