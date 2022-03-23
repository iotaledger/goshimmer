package evilspammer

import (
	"fmt"
	"time"
)

func DataSpammingFunction(s *Spammer) {
	clt := s.Clients.GetClient()
	msgID, err := clt.Data([]byte(fmt.Sprintf("SPAM")))
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
	// choose two different node to prevent being blocked
	txs, err := s.SpamWallet.PrepareDoubleSpendTransactions(s.EvilScenario)
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
			s.PostTransaction(txs[i])
		})
	}

	s.CheckIfAllSent()
}

func CustomConflictSpammingFunc(s *Spammer) {
	//// choose two different node to prevent being blocked
	//clts := s.Clients.GetClients(s.NumberOfSpends)
	//txs, err := s.SpamWallet.PrepareCustomConflictsSpam(s.EvilScenario)
	//if err != nil {
	//	s.ErrCounter.CountError(ErrFailToPrepareTransaction)
	//	return
	//}

}
