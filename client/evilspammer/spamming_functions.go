package evilspammer

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
)

func DataSpammingFunction(s *Spammer) {
	clt := s.Clients.GetClient()
	// sleep randomly to avoid issuing messages in different goroutines at once
	//time.Sleep(time.Duration(rand.Float64()*100) * time.Millisecond)
	if s.UseRateSetter {
		if err := evilwallet.RateSetterSleep(clt); err != nil {
			s.ErrCounter.CountError(err)
		}
	}
	msgID, err := clt.PostData([]byte(fmt.Sprintf("SPAM")))
	if err != nil {
		s.ErrCounter.CountError(ErrFailSendDataMessage)
	}

	count := s.State.txSent.Add(1)
	if count%int64(s.SpamDetails.Rate*2) == 0 {
		s.log.Debugf("Last sent message, ID: %s; msgCount: %d", msgID, count)
	}
	s.State.batchPrepared.Add(1)
	s.CheckIfAllSent()
}

func CustomConflictSpammingFunc(s *Spammer) {
	conflictBatch, aliases, err := s.EvilWallet.PrepareCustomConflictsSpam(s.EvilScenario)
	if err != nil {
		s.log.Debugf(errors.Newf("%v: %w", ErrFailToPrepareBatch, err).Error())
		s.ErrCounter.CountError(errors.Newf("%v: %w", ErrFailToPrepareBatch, err))
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

				// sleep randomly to avoid issuing messages in different goroutines at once
				time.Sleep(time.Duration(rand.Float64()*100) * time.Millisecond)
				if s.UseRateSetter {
					if err = evilwallet.RateSetterSleep(clt); err != nil {
						s.ErrCounter.CountError(err)
					}
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
