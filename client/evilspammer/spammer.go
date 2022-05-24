package evilspammer

import (
	"time"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/types"
	"go.uber.org/atomic"
)

// region Spammer //////////////////////////////////////////////////////////////////////////////////////////////////////

type SpammerFunc func(*Spammer)

type State struct {
	spamTicker    *time.Ticker
	logTicker     *time.Ticker
	spamStartTime time.Time
	txSent        *atomic.Int64
	batchPrepared *atomic.Int64

	logTickTime  time.Duration
	spamDuration time.Duration
}

// Spammer is a utility object for new spammer creations, can be modified by passing options.
// Mandatory options: WithClients, WithSpammingFunc
// Not mandatory options, if not provided spammer will use default settings:
// WithSpamDetails, WithEvilWallet, WithErrorCounter, WithLogTickerInterval
type Spammer struct {
	SpamDetails *SpamDetails
	State       *State

	Clients      evilwallet.Connector
	EvilWallet   *evilwallet.EvilWallet
	EvilScenario *evilwallet.EvilScenario
	ErrCounter   *ErrorCounter
	log          Logger

	// accessed from spamming functions
	done     chan bool
	shutdown chan types.Empty
	spamFunc SpammerFunc

	TimeDelayBetweenConflicts time.Duration
	NumberOfSpends            int
}

// NewSpammer is a constructor of Spammer.
func NewSpammer(options ...Options) *Spammer {
	state := &State{
		txSent:        atomic.NewInt64(0),
		batchPrepared: atomic.NewInt64(0),
		logTickTime:   time.Second * 30,
	}
	s := &Spammer{
		SpamDetails:    &SpamDetails{},
		spamFunc:       CustomConflictSpammingFunc,
		State:          state,
		EvilScenario:   evilwallet.NewEvilScenario(),
		done:           make(chan bool),
		shutdown:       make(chan types.Empty),
		NumberOfSpends: 2,
	}

	for _, opt := range options {
		opt(s)
	}

	if s.EvilWallet == nil {
		s.EvilWallet = evilwallet.NewEvilWallet()
	}

	s.setup()
	return s
}

func (s *Spammer) MessagesSent() uint64 {
	return uint64(s.State.txSent.Load())
}

func (s *Spammer) BatchesPrepared() uint64 {
	return uint64(s.State.batchPrepared.Load())
}

func (s *Spammer) setup() {
	s.Clients = s.EvilWallet.Connector()

	s.setupSpamDetails()

	s.State.spamTicker = s.initSpamTicker()
	s.State.logTicker = s.initLogTicker()

	if s.log == nil {
		s.initLogger()
	}

	if s.ErrCounter == nil {
		s.ErrCounter = NewErrorCount()
	}
}

func (s *Spammer) setupSpamDetails() {
	if s.SpamDetails.Rate <= 0 {
		s.SpamDetails.Rate = 1
	}
	if s.SpamDetails.TimeUnit == 0 {
		s.SpamDetails.TimeUnit = time.Second
	}
	// provided only maxMsgSent, calculating the default max for maxDuration
	if s.SpamDetails.MaxDuration == 0 && s.SpamDetails.MaxBatchesSent > 0 {
		s.SpamDetails.MaxDuration = time.Hour * 100
	}
	// provided only maxDuration, calculating the default max for maxMsgSent
	if s.SpamDetails.MaxBatchesSent == 0 && s.SpamDetails.MaxDuration > 0 {
		s.SpamDetails.MaxBatchesSent = int(s.SpamDetails.MaxDuration.Seconds()/s.SpamDetails.TimeUnit.Seconds()*float64(s.SpamDetails.Rate)) + 1
	}
}

func (s *Spammer) initLogger() {
	config := configuration.New()
	_ = logger.InitGlobalLogger(config)
	logger.SetLevel(logger.LevelDebug)
	s.log = logger.NewLogger("Spammer")
}

func (s *Spammer) initSpamTicker() *time.Ticker {
	tickerTime := float64(s.SpamDetails.TimeUnit) / float64(s.SpamDetails.Rate)
	return time.NewTicker(time.Duration(tickerTime))
}

func (s *Spammer) initLogTicker() *time.Ticker {
	return time.NewTicker(s.State.logTickTime)
}

// Spam runs the spammer. Function will stop after maxDuration time will pass or when maxMsgSent will be exceeded
func (s *Spammer) Spam() {
	s.log.Infof("Start spamming transactions with %d rate", s.SpamDetails.Rate)

	s.State.spamStartTime = time.Now()
	timeExceeded := time.After(s.SpamDetails.MaxDuration)

	go func() {
		for {
			select {
			case <-s.State.logTicker.C:
				s.log.Infof("Messages issued so far: %d, errors encountered: %d", s.State.txSent.Load(), s.ErrCounter.GetTotalErrorCount())
			case <-timeExceeded:
				s.log.Infof("Maximum spam duration exceeded, stopping spammer....")
				s.StopSpamming()
				return
			case <-s.done:
				s.StopSpamming()
				return
			case <-s.State.spamTicker.C:
				go s.spamFunc(s)
			}
		}
	}()
	<-s.shutdown
	s.log.Info(s.ErrCounter.GetErrorsSummary())
	s.log.Infof("Finishing spamming, total txns sent: %v, TotalTime: %v, Rate: %f", s.State.txSent.Load(), s.State.spamDuration.Seconds(), float64(s.State.txSent.Load())/s.State.spamDuration.Seconds())
}

func (s *Spammer) CheckIfAllSent() {
	if s.State.batchPrepared.Load() >= int64(s.SpamDetails.MaxBatchesSent) {
		s.log.Infof("Maximum number of messages sent, stopping spammer...")
		s.done <- true
	}
}

// StopSpamming finishes tasks before shutting down the spammer
func (s *Spammer) StopSpamming() {
	s.State.spamDuration = time.Since(s.State.spamStartTime)
	s.State.spamTicker.Stop()
	s.State.logTicker.Stop()
	s.shutdown <- types.Void
}

// PostTransaction use provided client to issue a transaction. It chooses API method based on Spammer options. Counts errors,
// counts transactions and provides debug logs.
func (s *Spammer) PostTransaction(tx *ledgerstate.Transaction, clt evilwallet.Client) {
	if tx == nil {
		s.log.Debug(ErrTransactionIsNil)
		s.ErrCounter.CountError(ErrTransactionIsNil)
	}
	allSolid := s.handleSolidityForReuseOutputs(clt, tx)
	if !allSolid {
		s.log.Debug(ErrInputsNotSolid)
		s.ErrCounter.CountError(errors.Errorf("%v, txID: %s", ErrInputsNotSolid, tx.ID().Base58()))
		return
	}

	var err error
	var txID ledgerstate.TransactionID
	txID, err = clt.PostTransaction(tx)
	if err != nil {
		s.log.Debug(ErrFailPostTransaction)
		s.ErrCounter.CountError(errors.Newf("%s: %w", ErrFailPostTransaction, err))
		return
	}
	if s.EvilScenario.OutputWallet.Type() == evilwallet.Reuse {
		s.EvilWallet.SetTxOutputsSolid(tx.Essence().Outputs(), clt.Url())
	}

	count := s.State.txSent.Add(1)
	s.log.Debugf("Last transaction sent, ID: %s, txCount: %d", txID.String(), count)
	return
}

func (s *Spammer) handleSolidityForReuseOutputs(clt evilwallet.Client, tx *ledgerstate.Transaction) (ok bool) {
	ok = true
	ok = s.EvilWallet.AwaitInputsSolidity(tx.Essence().Inputs(), clt)
	if s.EvilScenario.OutputWallet.Type() == evilwallet.Reuse {
		s.EvilWallet.AddReuseOutputsToThePool(tx.Essence().Outputs())
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type Logger interface {
	Infof(template string, args ...interface{})
	Info(args ...interface{})
	Debugf(template string, args ...interface{})
	Debug(args ...interface{})
	Warn(args ...interface{})
	Warnf(template string, args ...interface{})
	Error(args ...interface{})
	Errorf(template string, args ...interface{})
}
