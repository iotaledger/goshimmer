package main

import (
	"time"

	"github.com/iotaledger/goshimmer-tools/internal-tools/standardized-tests/spammers"
	"github.com/iotaledger/goshimmer-tools/internal-tools/standardized-tests/tools"
)

type IncreasingDoubleSpendTestParams struct {
	ClientUrls            []string
	Start                 int
	Stop                  int
	Step                  int
	EpochDuration         time.Duration
	TimeUnit              time.Duration
	DelayBetweenConflicts time.Duration
	VerifyLedger          bool
}

type ReasonableDoubleSpendTestParams struct {
	ClientUrls            []string
	DataRate              int
	DsRate                int
	TimeUnit              time.Duration
	EpochDuration         time.Duration
	EpochsNumber          int
	DelayBetweenConflicts time.Duration
	VerifyLedger          bool
}

// IncreasingDoubleSpendTest spam with increased rate for each next epoch
func IncreasingDoubleSpendTest(params *IncreasingDoubleSpendTestParams) {
	clients := spammers.NewClientsFromURL(params.ClientUrls, "")
	count := spammers.NewErrorCount() // to count errors encountered during spam
	fundsNeeded := 0

	for rate := params.Start; rate <= params.Stop; rate += params.Step {
		fundsNeeded += int(params.EpochDuration/params.TimeUnit) * rate
	}
	log.Infof("Preparing wallets for at least %d outputs", fundsNeeded)
	// prepare funds for spamming
	spamWallets, err := spammers.PrepareFunds(clients, fundsNeeded, 5, count)
	if err != nil {
		log.Errorf("Could not prepare funds")
		return
	}
	dsSent := 0
	opt := []spammers.Options{
		spammers.WithClients(clients),
		spammers.WithSpammingFunc(spammers.DoubleSpendSpammingFunc, true),
		spammers.WithErrorCounter(count),
		spammers.WithSpamWallet(spamWallets),
		spammers.WithTimeDelayForDoubleSpend(params.DelayBetweenConflicts),
	}
	for rate := params.Start; rate <= params.Stop; rate += params.Step {
		options := append(opt, spammers.WithSpamDetails(rate, params.TimeUnit, params.EpochDuration, 0))
		spammer := spammers.NewSpammer(options...)

		log.Infof("Double spend spamming with rate %d per %s params.Started\n", rate, params.TimeUnit.String())
		spammer.Spam()

		dsSent += int(params.EpochDuration/params.TimeUnit) * rate / 2
		// Wait a little and check if all nodes are still synced
		log.Infof("already sent about %d double spends, waiting 5sec before next test\n", dsSent)
		time.Sleep(time.Second * 5)
		syncStats, allSynced := tools.GetSyncStatus(clients)
		log.Infof("Sync status: %v", syncStats)
		if !allSynced {
			log.Errorf("Not all nodes synced!, sync status: %v", syncStats)
			return
		}
	}
	log.Info(count.GetErrorsSummary())
	log.Info("Double spend test finished, waiting 10s before verifying the ledger state")
	if params.VerifyLedger {
		log.Infof("Ledger verification params.Starts within 10s...")
		time.Sleep(time.Second * 10)
		tools.VerifyBranches(clients)
	}
}

func ReasonableDoubleSpendTest(params *ReasonableDoubleSpendTestParams) {
	clients := spammers.NewClientsFromURL(params.ClientUrls, "")

	for epoch := 0; epoch < params.EpochsNumber; epoch++ {
		log.Infof("New epoch %d of spamming params.Started, preparing funds...", epoch)
		// spam for one hour
		count := spammers.NewErrorCount() // to count errors encountered during spam
		outWallet := spammers.NewWallets()
		baseOptions := []spammers.Options{
			spammers.WithErrorCounter(count),
			spammers.WithClients(clients),
		}
		msgOptions := append(baseOptions,
			spammers.WithSpamDetails(params.DataRate, params.TimeUnit, params.EpochDuration, 0),
			spammers.WithSpammingFunc(spammers.MessageSpammingFunc, false),
		)
		dsOptions := append(baseOptions,
			spammers.WithSpamDetails(params.DsRate, params.TimeUnit, params.EpochDuration, 0),
			spammers.WithSpammingFunc(spammers.DoubleSpendSpammingFunc, true),
			spammers.WithTimeDelayForDoubleSpend(params.DelayBetweenConflicts),
			spammers.WithOutputWallets(outWallet),
		)

		msgSpammer := spammers.NewSpammer(msgOptions...)
		dsSpammer := spammers.NewSpammer(dsOptions...)

		go msgSpammer.Spam()
		dsSpammer.Spam()

		log.Info(count.GetErrorsSummary())
		log.Infof("Hour: %d of spamming finished", epoch+1)

		syncStats, allSynced := tools.GetSyncStatus(clients)
		log.Infof("Sync status: %v", syncStats)
		if !allSynced {
			log.Error("Not all nodes synced!")
		}

	}

	log.Info("Double spend test finished")
	if params.VerifyLedger {
		log.Infof("Ledger verification params.Starts within 10s...")
		time.Sleep(time.Second * 10)
		tools.VerifyBranches(clients)
	}
}
