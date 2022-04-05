package evilspammer

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
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
	s.State.batchPrepared.Add(1)
	s.CheckIfAllSent()
}

func CustomConflictSpammingFunc(s *Spammer) {
	conflictBatch, aliases, err := s.EvilWallet.PrepareCustomConflictsSpam(s.EvilScenario)
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
				allSolid := handleSolidityForReuseOutputs(clt, tx, s)
				if !allSolid {
					s.ErrCounter.CountError(errors.Errorf("not all inputs are solid, txID: %s", tx.ID().Base58()))
					return
				}
				ok := s.PostTransaction(tx, clt)
				if ok {
					if s.EvilScenario.OutputWallet.Type() == evilwallet.Reuse {
						s.EvilWallet.SetTxOutputsSolid(tx.Essence().Outputs(), clt.Url())
					}
				}
			}(clients[i], tx)
		}
		wg.Wait()
	}
	s.State.batchPrepared.Add(1)
	s.EvilWallet.ClearAliases(aliases)
	s.CheckIfAllSent()
}

func handleSolidityForReuseOutputs(clt evilwallet.Client, tx *ledgerstate.Transaction, s *Spammer) (ok bool) {
	ok = true
	if s.EvilScenario.Reuse {
		ok = s.EvilWallet.AwaitInputsSolidity(tx.Essence().Inputs(), clt)
	}
	if s.EvilScenario.OutputWallet.Type() == evilwallet.Reuse {
		s.EvilWallet.AddReuseOutputsToThePool(tx.Essence().Outputs())
	}
	return
}
