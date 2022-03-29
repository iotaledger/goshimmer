package evilspammer

import (
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"sync"
	"time"
)

func DataSpammingFunction(s *Spammer) {
	clt := s.Clients.GetClient()
	msgID, err := clt.PostData([]byte(fmt.Sprintf("SPAM")))
	if err != nil {
		s.ErrCounter.CountError(ErrFailSendDataMessage)
	}
	count := s.State.txSent.Add(1)
	if count%int64(s.SpamDetails.Rate*2) == 0 {
		s.log.Debugf("Last sent message, ID: %s; msgCount: %d", msgID, count)
	}

	s.CheckIfAllSent()
}

func ValueSpammingFunc(s *Spammer) {
	tx, err := s.SpamWallet.PrepareTransaction(s.EvilScenario)
	if err != nil {
		s.ErrCounter.CountError(ErrFailToPrepareTransaction)
		return
	}
	s.PostTransaction(tx)
	s.CheckIfAllSent()
}


func CustomConflictSpammingFunc(s *Spammer) {
	conflictBatch, err := s.SpamWallet.PrepareCustomConflicts(s.EvilScenario.ConflictBatch, s.EvilScenario.OutputWallet)
	if err != nil {
		s.ErrCounter.CountError(errors.Newf("custom conflict batch could not be prepared: %w", err))
	}
	for _, txs := range conflictBatch {
		clients := s.Clients.GetClients(len(txs))
		if len(txs) > len(clients) {
			s.ErrCounter.CountError(errors.New("insufficient clients to send conflicts"))
		}

		// send transactions in parallel
		wg := sync.WaitGroup{}
		for i, tx := range txs {
			wg.Add(1)
			go func(clt evilwallet.Client, tx *ledgerstate.Transaction) {
				defer wg.Done()
				_, _ = clt.PostTransaction(tx)
			}(clients[i], tx)
		}
		wg.Wait()
	}

}
