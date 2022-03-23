package evilspammer

import (
	"time"

	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/types"
	"go.uber.org/atomic"
)

// region Spammer //////////////////////////////////////////////////////////////////////////////////////////////////////

// todo add possibility to reuse Spammer instance, pause, restart, resume  and change Evil scenario

type SpammerFunc func(*Spammer)

type State struct {
	spamTicker    *time.Ticker
	logTicker     *time.Ticker
	spamStartTime time.Time
	txSent        *atomic.Int64
	txFailed      *atomic.Int64

	logTickTime  time.Duration
	spamDuration time.Duration
}

// Spammer is a utility object for new spammer creations, can be modified by passing options.
// Mandatory options: WithClients, WithSpammingFunc
// Not mandatory options, if not provided spammer will use default settings:
// WithSpamDetails, WithSpamWallet, WithErrorCounter, WithLogTickerInterval
type Spammer struct {
	SpamDetails *SpamDetails
	State       *State

	Clients      evilwallet.Connector
	SpamWallet   *evilwallet.EvilWallet
	EvilScenario evilwallet.EvilScenario
	ErrCounter   *ErrorCounter
	log          Logger

	// accessed from spamming functions
	done     chan bool
	shutdown chan types.Empty
	spamFunc SpammerFunc

	TimeDelayBetweenConflicts time.Duration
	NumberOfSpends            int
}

// NewSpammer constructor of Spammer
func NewSpammer(options ...Options) *Spammer {
	state := &State{
		txSent:      atomic.NewInt64(0),
		txFailed:    atomic.NewInt64(0),
		logTickTime: time.Second * 30,
	}
	s := &Spammer{
		SpamDetails:    DefaultSpamDetails,
		State:          state,
		done:           make(chan bool),
		shutdown:       make(chan types.Empty),
		NumberOfSpends: 2,
	}

	for _, opt := range options {
		opt(s)
	}

	s.setup()
	s.Clients = s.SpamWallet.Connector()
	return s
}

func (s *Spammer) setup() {
	s.Clients = s.SpamWallet.Connector()

	if s.SpamDetails.Rate <= 0 {
		s.SpamDetails.Rate = 1
	}
	s.State.spamTicker = s.initSpamTicker()
	s.State.logTicker = s.initLogTicker()

	if s.log == nil {
		s.initLogger()
	}

	if s.ErrCounter == nil {
		s.ErrCounter = NewErrorCount()
	}
}

func (s *Spammer) initLogger() {
	config := configuration.New()
	if err := logger.InitGlobalLogger(config); err != nil {
		panic(err)
	}
	logger.SetLevel(logger.LevelInfo)
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
				s.log.Infof("Maximum spam duration exceeded, stoping spammer....")
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
	if s.State.txSent.Load()+s.State.txFailed.Load() >= int64(s.SpamDetails.MaxMsgSent) {
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
func (s *Spammer) PostTransaction(tx *ledgerstate.Transaction) (success bool) {
	if tx == nil {
		s.log.Debugf("transaction provided to PostTransaction is nil")
		s.ErrCounter.CountError(ErrTransactionIsNil)
	}

	var err error
	var txID ledgerstate.TransactionID
	clt := s.Clients.GetClient()
	txID, err = clt.PostTransaction(tx)
	if err != nil {
		s.log.Debugf("error: %v", err)
		s.ErrCounter.CountError(ErrFailPostTransaction)
		s.State.txFailed.Add(1)
		return
	}
	count := s.State.txSent.Add(1)
	s.log.Debugf("Last transaction sent, ID: %s, txCount: %d", txID.String(), count)
	return true
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
