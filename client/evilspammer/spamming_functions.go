package evilspammer

import (
	"math/rand"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
)

func DataSpammingFunction(s *Spammer) {
	clt := s.Clients.GetClient()
	// sleep randomly to avoid issuing blocks in different goroutines at once
	time.Sleep(time.Duration(rand.Float64()*20) * time.Millisecond)
	if err := evilwallet.RateSetterSleep(clt, s.UseRateSetter); err != nil {
		s.ErrCounter.CountError(err)
	}
	blkID, err := clt.PostData([]byte("SPAM"))
	if err != nil {
		s.ErrCounter.CountError(ErrFailSendDataBlock)
	}

	count := s.State.txSent.Add(1)
	if count%int64(s.SpamDetails.Rate*2) == 0 {
		s.log.Debugf("Last sent block, ID: %s; blkCount: %d", blkID, count)
	}
	s.State.batchPrepared.Add(1)
	s.CheckIfAllSent()
}

func CustomConflictSpammingFunc(s *Spammer) {
	conflictBatch, aliases, err := s.EvilWallet.PrepareCustomConflictsSpam(s.EvilScenario)
	if err != nil {
		s.log.Debugf(errors.WithMessage(ErrFailToPrepareBatch, err.Error()).Error())
		s.ErrCounter.CountError(errors.WithMessage(ErrFailToPrepareBatch, err.Error()))
	}

	for _, txs := range conflictBatch {
		clients := s.Clients.GetClients(len(txs))
		if len(txs) > len(clients) {
			s.log.Debug(ErrFailToPrepareBatch)
			s.ErrCounter.CountError(ErrInsufficientClients)
		}

		// send transactions in parallel
		wg := sync.WaitGroup{}
		for i, tx := range txs {
			wg.Add(1)
			go func(clt evilwallet.Client, tx *devnetvm.Transaction) {
				defer wg.Done()

				// sleep randomly to avoid issuing blocks in different goroutines at once
				time.Sleep(time.Duration(rand.Float64()*100) * time.Millisecond)
				if err = evilwallet.RateSetterSleep(clt, s.UseRateSetter); err != nil {
					s.ErrCounter.CountError(err)
				}
				s.PostTransaction(tx, clt)
			}(clients[i], tx)
		}
		wg.Wait()
	}
	s.State.batchPrepared.Add(1)
	s.EvilWallet.ClearAliases(aliases)
	s.CheckIfAllSent()
}
