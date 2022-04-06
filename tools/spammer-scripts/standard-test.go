package main

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer-tools/internal-tools/standardized-tests/spammers"
	"github.com/iotaledger/goshimmer/client"
)

// StandardTest is spamming the network with different proportion of data and value messages for increasing periods of time and mps
func StandardTest(clients *spammers.Clients) {
	count := spammers.NewErrorCount() // to count errors encountered during spam

	opt := []spammers.Options{
		spammers.WithClients(clients),
		spammers.WithErrorCounter(count),
	}

	for seconds := 20; seconds <= 60; seconds += 20 {
		for mps := 100; mps <= 200; mps += 20 {
			duration := time.Duration(seconds) * time.Second

			// prepare funds for spamming
			fundsNeeded := calculateWalletSize(mps, seconds)
			spamWallets, err := spammers.PrepareFunds(clients, fundsNeeded, 5, count)
			if err != nil {
				log.Errorf("Could not prepare funds, mps: %d, seconds: %d", mps, seconds)
			}
			for p := 0.1; p < 1; p += 0.2 {
				dMps := int(float64(mps) * (1 - p))
				vMps := int(float64(mps) * p)

				msgOpt := append(opt, []spammers.Options{
					spammers.WithSpammingFunc(spammers.MessageSpammingFunc, false),
					spammers.WithSpamDetails(dMps, time.Second, duration, 0),
				}...)
				msgSpammer := spammers.NewSpammer(msgOpt...)

				txOpt := append(opt, []spammers.Options{
					spammers.WithSpammingFunc(spammers.TransactionSpammingFunc, true),
					spammers.WithSpamDetails(vMps, time.Second, duration, 0),
					spammers.WithSpamWallet(spamWallets),
				}...)
				txSpammer := spammers.NewSpammer(txOpt...)

				log.Infof("Starting test with, data-mps: %d, value-mps:%d, spam duration in sec: %v", dMps, vMps, duration.Seconds())
				clt := clients.GetClients(1)[0]
				awaitTipPoolSizeIsSmallEnough(clt, time.Minute*2, 20)

				wg := sync.WaitGroup{}
				wg.Add(2)
				go func(dMps int, d time.Duration) {
					defer wg.Done()
					msgSpammer.Spam()
				}(dMps, duration)
				go func(vMps int, d time.Duration) {
					defer wg.Done()
					txSpammer.Spam()
				}(vMps, duration)
				wg.Wait()
			}
		}
	}
	log.Info(count.GetErrorsSummary())
	log.Info("Standard test finished")
}

// calculates how many wallets containing 10k unspent outputs should be created to perform spam with provided mps and duration
func calculateWalletSize(mps int, seconds int) int {
	totalSize := 0
	for i := 0; i < 10; i++ {
		totalSize += mps * i / 10 * seconds
	}
	return totalSize
}

// awaitTipPoolSizeIsSmallEnough await the tip pool size is small enough
func awaitTipPoolSizeIsSmallEnough(clt *client.GoShimmerAPI, waitFor time.Duration, maxTipPoolSize int) {
	s := time.Now()
	for ; time.Since(s) < waitFor; time.Sleep(5 * time.Second) {
		resp, err := clt.GetDiagnosticsTips()
		if err != nil {
			log.Errorf("could not get tip response: %v", err)
		}

		numOfLines, err := spammers.LineCount(resp)
		if numOfLines <= maxTipPoolSize {
			log.Infof("Number of tips reduced to %d, cooling down finished", numOfLines-1)
			break
		}
	}
	return
}
