package evilspammer

import (
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"time"
)

type Options func(*Spammer)

// WithSpamDetails provides spammer with options regarding rate, time unit, and finishing spam criteria. Provide 0 to one of max parameters to skip it.
func WithSpamDetails(rate int, timeUnit time.Duration, maxDuration time.Duration, maxMsgSent int) Options {
	// provided only maxMsgSent, calculating the default max for maxDuration
	if maxDuration == 0 && maxMsgSent > 0 {
		maxDuration = time.Hour
	}
	// provided only maxDuration, calculating the default max for maxMsgSent
	if maxMsgSent == 0 && maxDuration > 0 {
		maxMsgSent = int(maxDuration/timeUnit)*rate + 1
	}

	return func(s *Spammer) {
		s.SpamDetails = &SpamDetails{
			Rate:        rate,
			TimeUnit:    timeUnit,
			MaxDuration: maxDuration,
			MaxMsgSent:  maxMsgSent,
		}
	}
}

// WithSpamWallet provides evil wallet instance, that will handle all spam logic according to provided EvilScenario
func WithSpamWallet(initWallets *evilwallet.EvilWallet) Options {
	return func(s *Spammer) {
		s.SpamWallet = initWallets
	}
}

// WithEvilScenario provides initWallet of spammer, if omitted spammer will prepare funds based on maxMsgSent parameter
func WithEvilScenario(scenario evilwallet.EvilScenario) Options {
	return func(s *Spammer) {
		s.EvilScenario = scenario
	}
}

// WithErrorCounter allows for setting an error counter object, if not provided a new instance will be created.
func WithErrorCounter(errCounter *ErrorCounter) Options {
	return func(s *Spammer) {
		s.ErrCounter = errCounter
	}
}

// WithLogTickerInterval allows for changing interval between progress spamming logs, default is 30s.
func WithLogTickerInterval(interval time.Duration) Options {
	return func(s *Spammer) {
		s.State.logTickTime = interval
	}
}

// WithSpammingFunc sets core function of the spammer with spamming logic, needs to use done spammer's channel to communicate.
// end of spamming and errors.
func WithSpammingFunc(spammerFunc func(s *Spammer)) Options {
	return func(s *Spammer) {
		s.spamFunc = spammerFunc
	}
}

func WithTimeDelayForDoubleSpend(timeDelay time.Duration) Options {
	return func(s *Spammer) {
		s.TimeDelayBetweenConflicts = timeDelay
	}
}

// WithNumberOfSpends sets how many transactions should be created with the same input, e.g 3 for triple spend,
// 2 for double spend. For this to work user needs to make sure that there is enough number of clients.
func WithNumberOfSpends(n int) Options {
	return func(s *Spammer) {
		s.NumberOfSpends = n
	}
}

type SpamDetails struct {
	Rate        int
	TimeUnit    time.Duration
	MaxDuration time.Duration
	MaxMsgSent  int
}

var DefaultSpamDetails = &SpamDetails{
	Rate:        10,
	TimeUnit:    time.Second,
	MaxDuration: time.Minute,
	MaxMsgSent:  601,
}
