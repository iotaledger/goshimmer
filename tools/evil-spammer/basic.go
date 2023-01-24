package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/client/evilspammer"
	"github.com/iotaledger/goshimmer/client/evilwallet"
)

type CustomSpamParams struct {
	ClientURLs            []string
	SpamTypes             []string
	Rates                 []int
	Durations             []time.Duration
	BlkToBeSent           []int
	TimeUnit              time.Duration
	DelayBetweenConflicts time.Duration
	NSpend                int
	Scenario              evilwallet.EvilBatch
	DeepSpam              bool
	EnableRateSetter      bool
}

func CustomSpam(params *CustomSpamParams) {
	wallet := evilwallet.NewEvilWallet(params.ClientURLs...)
	wg := sync.WaitGroup{}

	fundsNeeded := false
	for _, st := range params.SpamTypes {
		if st != "blk" {
			fundsNeeded = true
		}
	}
	if fundsNeeded {
		err := wallet.RequestFreshBigFaucetWallet()
		if err != nil {
			panic(err)
		}
	}


	for i, sType := range params.SpamTypes {
		log.Infof("Start spamming with rate: %d, time unit: %s, and spamming type: %s.", params.Rates[i], params.TimeUnit.String(), sType)


		switch sType {
		case "blk":
			wg.Add(1)
			go func() {
				defer wg.Done()
				s := SpamBlocks(wallet, params.Rates[i], params.TimeUnit, params.Durations[i], params.BlkToBeSent[i], params.EnableRateSetter)
				if s == nil {
					return
				}
				s.Spam()
			}()
		case "tx":
			wg.Add(1)
			go func() {
				defer wg.Done()
				SpamTransaction(wallet, params.Rates[i], params.TimeUnit, params.Durations[i], params.DeepSpam, params.EnableRateSetter)
			}()
		case "ds":
			wg.Add(1)
			go func() {
				defer wg.Done()
				SpamDoubleSpends(wallet, params.Rates[i], params.NSpend, params.TimeUnit, params.Durations[i], params.DelayBetweenConflicts, params.DeepSpam, params.EnableRateSetter)
			}()
		case "custom":
			wg.Add(1)
			go func() {
				defer wg.Done()
				s := SpamNestedConflicts(wallet, params.Rates[i], params.TimeUnit, params.Durations[i], params.Scenario, params.DeepSpam, false, params.EnableRateSetter)
				if s == nil {
					return
				}
				s.Spam()
			}()

		default:
			log.Warn("Spamming type not recognized. Try one of following: tx, ds, blk")
		}
	}

	wg.Wait()
	log.Info("Basic spamming finished!")
}

func SpamTransaction(wallet *evilwallet.EvilWallet, rate int, timeUnit, duration time.Duration, deepSpam, enableRateSetter bool) {
	if wallet.NumOfClient() < 1 {
		printer.NotEnoughClientsWarning(1)
	}

	scenarioOptions := []evilwallet.ScenarioOption{
		evilwallet.WithScenarioCustomConflicts(evilwallet.SingleTransactionBatch()),
	}
	if deepSpam {
		outWallet := wallet.NewWallet(evilwallet.Reuse)
		scenarioOptions = append(scenarioOptions,
			evilwallet.WithScenarioDeepSpamEnabled(),
			evilwallet.WithScenarioReuseOutputWallet(outWallet),
			evilwallet.WithScenarioInputWalletForDeepSpam(outWallet),
		)
	}
	scenarioTx := evilwallet.NewEvilScenario(scenarioOptions...)

	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithRateSetter(enableRateSetter),
		evilspammer.WithEvilWallet(wallet),
		evilspammer.WithEvilScenario(scenarioTx),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}

func SpamDoubleSpends(wallet *evilwallet.EvilWallet, rate, nSpent int, timeUnit, duration, delayBetweenConflicts time.Duration, deepSpam, enableRateSetter bool) {
	log.Debugf("Setting up double spend spammer with rate: %d, time unit: %s, and duration: %s.", rate, timeUnit.String(), duration.String())
	if wallet.NumOfClient() < 2 {
		printer.NotEnoughClientsWarning(2)
	}

	scenarioOptions := []evilwallet.ScenarioOption{
		evilwallet.WithScenarioCustomConflicts(evilwallet.NSpendBatch(nSpent)),
	}
	if deepSpam {
		outWallet := wallet.NewWallet(evilwallet.Reuse)
		scenarioOptions = append(scenarioOptions,
			evilwallet.WithScenarioDeepSpamEnabled(),
			evilwallet.WithScenarioReuseOutputWallet(outWallet),
			evilwallet.WithScenarioInputWalletForDeepSpam(outWallet),
		)
	}
	scenarioDs := evilwallet.NewEvilScenario(scenarioOptions...)
	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithEvilWallet(wallet),
		evilspammer.WithRateSetter(enableRateSetter),
		evilspammer.WithTimeDelayForDoubleSpend(delayBetweenConflicts),
		evilspammer.WithEvilScenario(scenarioDs),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}

func SpamNestedConflicts(wallet *evilwallet.EvilWallet, rate int, timeUnit, duration time.Duration, conflictBatch evilwallet.EvilBatch, deepSpam, reuseOutputs, enableRateSetter bool) (spammer *evilspammer.Spammer) {
	scenarioOptions := []evilwallet.ScenarioOption{
		evilwallet.WithScenarioCustomConflicts(conflictBatch),
	}
	if deepSpam {
		outWallet := wallet.NewWallet(evilwallet.Reuse)
		scenarioOptions = append(scenarioOptions,
			evilwallet.WithScenarioDeepSpamEnabled(),
			evilwallet.WithScenarioReuseOutputWallet(outWallet),
			evilwallet.WithScenarioInputWalletForDeepSpam(outWallet),
		)
	} else if reuseOutputs {
		outWallet := wallet.NewWallet(evilwallet.Reuse)
		scenarioOptions = append(scenarioOptions, evilwallet.WithScenarioReuseOutputWallet(outWallet))
	}
	scenario := evilwallet.NewEvilScenario(scenarioOptions...)
	if scenario.NumOfClientsNeeded > wallet.NumOfClient() {
		spammer = nil
		printer.NotEnoughClientsWarning(scenario.NumOfClientsNeeded)
	}

	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithEvilWallet(wallet),
		evilspammer.WithRateSetter(enableRateSetter),
		evilspammer.WithEvilScenario(scenario),
	}

	return evilspammer.NewSpammer(options...)
}

func SpamBlocks(wallet *evilwallet.EvilWallet, rate int, timeUnit, duration time.Duration, numBlkToSend int, enableRateSetter bool) *evilspammer.Spammer {
	if wallet.NumOfClient() < 1 {
		printer.NotEnoughClientsWarning(1)
	}

	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithBatchesSent(numBlkToSend),
		evilspammer.WithRateSetter(enableRateSetter),
		evilspammer.WithEvilWallet(wallet),
		evilspammer.WithSpammingFunc(evilspammer.DataSpammingFunction),
	}
	spammer := evilspammer.NewSpammer(options...)
	return spammer
}
