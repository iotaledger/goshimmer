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
	Durations             []time.Duration
	MsgToBeSent           []int
	TimeUnit              time.Duration
	DelayBetweenConflicts time.Duration
	NSpend                int
	Scenario              evilwallet.EvilBatch
	DeepSpam              bool
}

func CustomSpam(params *CustomSpamParams) {
	wallet := evilwallet.NewEvilWallet(params.ClientUrls...)
	wg := sync.WaitGroup{}

	fundsNeeded := false
	for _, spamType := range params.SpamTypes {
		if spamType != "msg" {
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
				s := SpamMessages(wallet, params.Rates[i], params.TimeUnit, params.Durations[i], params.MsgToBeSent[i])
				if s == nil {
					return
				}
				s.Spam()
			}()
		case "tx":
			wg.Add(1)
			go func() {
				defer wg.Done()
				SpamTransaction(wallet, params.Rates[i], params.TimeUnit, params.Durations[i], params.DeepSpam)
			}()
		case "ds":
			wg.Add(1)
			go func() {
				defer wg.Done()
				SpamDoubleSpends(wallet, params.Rates[i], params.MsgToBeSent[i], params.TimeUnit, params.Durations[i], params.DelayBetweenConflicts, params.DeepSpam)
			}()
		case "nds":
			wg.Add(1)
			go func() {
				defer wg.Done()
				SpamNDoubleSpends(wallet, params.Rates[i], params.NSpend, params.TimeUnit, params.Durations[i], params.DelayBetweenConflicts, params.DeepSpam)
			}()
		case "custom":
			wg.Add(1)
			go func() {
				defer wg.Done()
				s := SpamNestedConflicts(wallet, params.Rates[i], params.TimeUnit, params.Durations[i], params.Scenario, params.DeepSpam, false)
				if s == nil {
					return
				}
				s.Spam()
			}()

		default:
			log.Warn("Spamming type not recognized. Try one of following: tx, ds, msg")
		}
	}

	wg.Wait()
	log.Info("Basic spamming finished!")
}

func SpamTransaction(wallet *evilwallet.EvilWallet, rate int, timeUnit, duration time.Duration, deepSpam bool) {
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
		evilspammer.WithEvilWallet(wallet),
		evilspammer.WithEvilScenario(scenarioTx),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}

func SpamDoubleSpends(wallet *evilwallet.EvilWallet, rate, numDsToSend int, timeUnit, duration, delayBetweenConflicts time.Duration, deepSpam bool) {
	if wallet.NumOfClient() < 2 {
		printer.NotEnoughClientsWarning(2)
	}

	scenarioOptions := []evilwallet.ScenarioOption{
		evilwallet.WithScenarioCustomConflicts(evilwallet.DoubleSpendBatch(numDsToSend)),
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
		evilspammer.WithTimeDelayForDoubleSpend(delayBetweenConflicts),
		evilspammer.WithEvilScenario(scenarioDs),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}

func SpamNDoubleSpends(wallet *evilwallet.EvilWallet, rate, nSpend int, timeUnit, duration, delayBetweenConflicts time.Duration, deepSpam bool) {
	if nSpend > wallet.NumOfClient() {
		printer.NotEnoughClientsWarning(nSpend)
	}

	scenarioOptions := []evilwallet.ScenarioOption{
		evilwallet.WithScenarioCustomConflicts(evilwallet.NSpendBatch(nSpend)),
	}
	if deepSpam {
		outWallet := wallet.NewWallet(evilwallet.Reuse)
		scenarioOptions = append(scenarioOptions,
			evilwallet.WithScenarioDeepSpamEnabled(),
			evilwallet.WithScenarioReuseOutputWallet(outWallet),
			evilwallet.WithScenarioInputWalletForDeepSpam(outWallet),
		)
	}
	scenario := evilwallet.NewEvilScenario(scenarioOptions...)
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

func SpamNestedConflicts(wallet *evilwallet.EvilWallet, rate int, timeUnit, duration time.Duration, conflictBatch evilwallet.EvilBatch, deepSpam, reuseOutputs bool) (spammer *evilspammer.Spammer) {
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
	} else {
		if reuseOutputs {
			outWallet := wallet.NewWallet(evilwallet.Reuse)
			scenarioOptions = append(scenarioOptions, evilwallet.WithScenarioReuseOutputWallet(outWallet))
		}
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
		evilspammer.WithEvilScenario(scenario),
	}
	spammer = evilspammer.NewSpammer(options...)

	return
}

func SpamMessages(wallet *evilwallet.EvilWallet, rate int, timeUnit, duration time.Duration, numMsgToSend int) *evilspammer.Spammer {
	if wallet.NumOfClient() < 1 {
		printer.NotEnoughClientsWarning(1)
	}

	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithBatchesSent(numMsgToSend),
		evilspammer.WithEvilWallet(wallet),
		evilspammer.WithSpammingFunc(evilspammer.DataSpammingFunction),
	}
	spammer := evilspammer.NewSpammer(options...)
	return spammer
}
