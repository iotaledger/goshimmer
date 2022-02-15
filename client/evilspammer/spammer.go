package evilspammer

import (
	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/types"
	"go.uber.org/atomic"
	"time"
)

// region Spammer //////////////////////////////////////////////////////////////////////////////////////////////////////

type SpammerFunc func(*Spammer)

type spammerState struct {
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
	SpamDetails  *SpamDetails
	spammerState *spammerState

	Clients     Clients
	InputFunds  Wallet
	OutputFunds Wallet
	errCounter  ErrorCounter
	log         Logger

	// accessed from spamming functions
	done     chan bool
	shutdown chan types.Empty
	spamFunc SpammerFunc

	IssuingAPIMethod          string
	TimeDelayBetweenConflicts time.Duration
}

// NewSpammer constructor of Spammer
func NewSpammer(options ...Options) *Spammer {
	state := &spammerState{
		txSent:      atomic.NewInt64(0),
		txFailed:    atomic.NewInt64(0),
		logTickTime: time.Second * 30,
	}
	s := &Spammer{
		SpamDetails:      DefaultSpamDetails,
		spammerState:     state,
		done:             make(chan bool),
		shutdown:         make(chan types.Empty),
		IssuingAPIMethod: "PostTransaction",
	}

	for _, opt := range options {
		opt(s)
	}

	s.setup()

	return s
}

func (s *Spammer) setup() {
	if s.SpamDetails.Rate <= 0 {
		s.SpamDetails.Rate = 1
	}
	s.spammerState.spamTicker = s.initSpamTicker()
	s.spammerState.logTicker = s.initLogTicker()
}

func (s *Spammer) initSpamTicker() *time.Ticker {
	tickerTime := float64(s.SpamDetails.TimeUnit) / float64(s.SpamDetails.Rate)
	return time.NewTicker(time.Duration(tickerTime))
}

func (s *Spammer) initLogTicker() *time.Ticker {
	return time.NewTicker(s.spammerState.logTickTime)
}

// Spam runs the spammer. Function will stop after maxDuration time will pass or when maxMsgSent will be exceeded
func (s *Spammer) Spam() {
	s.log.Infof("Start spamming transactions with %d rate", s.SpamDetails.Rate)

	s.spammerState.spamStartTime = time.Now()
	timeExceeded := time.After(s.SpamDetails.MaxDuration)

	go func() {
		for {
			select {
			case <-s.spammerState.logTicker.C:
				s.log.Infof("Messages issued so far: %d, errors encountered: %d", s.spammerState.txSent.Load(), s.errCounter.GetTotalErrorCount())
			case <-timeExceeded:
				s.log.Infof("Maximum spam duration exceeded, stoping spammer....")
				s.StopSpamming()
				return
			case <-s.done:
				s.StopSpamming()
				return
			case <-s.spammerState.spamTicker.C:
				go s.spamFunc(s)
			}
		}
	}()
	<-s.shutdown
	s.log.Info(s.errCounter.GetErrorsSummary())
	s.log.Infof("Finishing spamming, total txns sent: %v, TotalTime: %v, Rate: %f", s.spammerState.txSent.Load(), s.spammerState.spamDuration.Seconds(), float64(s.spammerState.txSent.Load())/s.spammerState.spamDuration.Seconds())
}

func (s *Spammer) CheckIfAllSent() {
	if s.spammerState.txSent.Load()+s.spammerState.txFailed.Load() >= int64(s.SpamDetails.MaxMsgSent) {
		s.log.Infof("Maximum number of messages sent, stopping spammer...")
		s.done <- true
	}
}

// StopSpamming finishes tasks before shutting down the spammer
func (s *Spammer) StopSpamming() {
	s.spammerState.spamDuration = time.Since(s.spammerState.spamStartTime)
	s.spammerState.spamTicker.Stop()
	s.spammerState.logTicker.Stop()
	s.shutdown <- types.Void
}

// PostTransaction use provided client to issue a transaction. It chooses API method based on Spammer options. Counts errors,
// counts transactions and provides debug logs.
func (s *Spammer) PostTransaction(tx *ledgerstate.Transaction, clt *client.GoShimmerAPI) (success bool) {
	if tx == nil {
		s.log.Debugf("transaction provided to PostTransaction is nil")
		s.errCounter.CountError(ErrTransactionIsNil)
	}

	var err error
	var txID *jsonmodels.PostTransactionResponse
	var msgID string

	switch s.IssuingAPIMethod {
	case "PostTransaction":
		txID, err = clt.PostTransaction(tx.Bytes())
	case "SendPayload":
		msgID, err = clt.SendPayload(tx.Bytes())
	}
	if err != nil {
		s.log.Debugf("error: %v", err)
		s.errCounter.CountError(ErrFailPostTransaction)
		s.spammerState.txFailed.Add(1)
		return
	}
	count := s.spammerState.txSent.Add(1)
	switch s.IssuingAPIMethod {
	case "PostTransaction":
		s.log.Debugf("Last transaction sent, ID: %s, txCount: %d", txID.TransactionID, count)
	case "SendPayload":
		s.log.Debugf("Last transaction sent, ID: %s, txCount: %d", msgID, count)
	}
	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type Wallet interface {
}

type Clients interface {
}

type Logger interface {
	Infof(template string, args ...interface{})
	Info(template string, args ...interface{})
	Debugf(template string, args ...interface{})
	Debug(template string, args ...interface{})
	Warn(template string, args ...interface{})
	Warnf(template string, args ...interface{})
	Error(template string, args ...interface{})
	Errorf(template string, args ...interface{})
}

type ErrorCounter interface {
	CountError(err error)
	GetErrorsSummary() string
	GetTotalErrorCount() int
}
