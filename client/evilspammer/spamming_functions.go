package evilspammer

import (
	"fmt"
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

func DoubleSpendSpammingFunc(s *Spammer) {
	// NOTE: need to set double spend EvilBatch before
	// choose two different node to prevent being blocked
	clts := s.Clients.GetClients(2)
	txs, err := s.SpamWallet.PrepareCustomConflictsSpam(s.EvilScenario)
	if err != nil {
		s.ErrCounter.CountError(ErrFailToPrepareTransaction)
		return
	}
	delays := make([]time.Duration, s.NumberOfSpends)
	d := time.Duration(0)
	for i := range delays {
		delays[i] = d
		d += s.TimeDelayBetweenConflicts
	}
	for i, delay := range delays {
		time.AfterFunc(delay, func() {
			_, err1 := clts[0].PostTransaction(txs[i][0])
			s.log.Error(err1)
			_, err2 := clts[1].PostTransaction(txs[i][1])
			s.log.Error(err2)
		})
	}

	s.CheckIfAllSent()
}

func CustomConflictSpammingFunc(s *Spammer) {
	txs, err := s.SpamWallet.PrepareCustomConflictsSpam(s.EvilScenario)
	if err != nil {
		s.ErrCounter.CountError(ErrFailToPrepareTransaction)
		return
	}
	delays := make([]time.Duration, s.NumberOfSpends)
	d := time.Duration(0)
	for i := range delays {
		delays[i] = d
		d += s.TimeDelayBetweenConflicts
	}
	for i, delay := range delays {
		time.AfterFunc(delay, func() {
			for _, tx := range txs[i] {
				s.PostTransaction(tx)
			}
		})
	}
	s.CheckIfAllSent()
}
