package evilspammer

import (
	"time"

	"github.com/iotaledger/goshimmer/client/evilwallet"
)

type Options func(*Spammer)

// region Spammer general options ///////////////////////////////////////////////////////////////////////////////////////////////////

// WithSpamRate provides spammer with options regarding rate, time unit, and finishing spam criteria. Provide 0 to one of max parameters to skip it.
func WithSpamRate(rate int, timeUnit time.Duration) Options {
	return func(s *Spammer) {
		if s.SpamDetails == nil {
			s.SpamDetails = &SpamDetails{
				Rate:     rate,
				TimeUnit: timeUnit,
			}
		} else {
			s.SpamDetails.Rate = rate
			s.SpamDetails.TimeUnit = timeUnit
		}
	}
}

// WithSpamDuration provides spammer with options regarding rate, time unit, and finishing spam criteria. Provide 0 to one of max parameters to skip it.
func WithSpamDuration(maxDuration time.Duration) Options {
	return func(s *Spammer) {
		if s.SpamDetails == nil {
			s.SpamDetails = &SpamDetails{
				MaxDuration: maxDuration,
			}
		} else {
			s.SpamDetails.MaxDuration = maxDuration
		}
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
// end of spamming and errors. Default one is the CustomConflictSpammingFunc.
func WithSpammingFunc(spammerFunc func(s *Spammer)) Options {
	return func(s *Spammer) {
		s.spamFunc = spammerFunc
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Spammer EvilWallet options ///////////////////////////////////////////////////////////////////////////////////////////////////

// WithRateSetter enables setting rate of spammer.
func WithRateSetter(enable bool) Options {
	return func(s *Spammer) {
		s.UseRateSetter = enable
	}
}

// WithBatchesSent provides spammer with options regarding rate, time unit, and finishing spam criteria. Provide 0 to one of max parameters to skip it.
func WithBatchesSent(maxBatchesSent int) Options {
	return func(s *Spammer) {
		if s.SpamDetails == nil {
			s.SpamDetails = &SpamDetails{
				MaxBatchesSent: maxBatchesSent,
			}
		} else {
			s.SpamDetails.MaxBatchesSent = maxBatchesSent
		}
	}
}

// WithEvilWallet provides evil wallet instance, that will handle all spam logic according to provided EvilScenario.
func WithEvilWallet(initWallets *evilwallet.EvilWallet) Options {
	return func(s *Spammer) {
		s.EvilWallet = initWallets
	}
}

// WithEvilScenario provides initWallet of spammer, if omitted spammer will prepare funds based on maxBlkSent parameter.
func WithEvilScenario(scenario *evilwallet.EvilScenario) Options {
	return func(s *Spammer) {
		s.EvilScenario = scenario
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Spammer Commitment options ///////////////////////////////////////////////////////////////////////////////////////////////////

func WithClientURLs(clientURLs []string) Options {
	return func(s *Spammer) {
		s.Clients = evilwallet.NewWebClients(clientURLs)
	}
}

func WithValidClientURL(validClient string) Options {
	return func(s *Spammer) {
		s.CommitmentManager.Params.ValidClientURL = validClient
	}
}

func WithIdentity(alias, privateKey string) Options {
	return func(s *Spammer) {
		s.IdentityManager.AddIdentity(privateKey, alias)
		s.IdentityManager.primaryAlias = alias
	}
}

// WithCommitmentType provides commitment type for the spammer, allowed types: fork, valid, random. Enables commitment spam and disables the wallet functionality.
func WithCommitmentType(commitmentType string) Options {
	return func(s *Spammer) {
		s.SpamType = SpamCommitments
		s.CommitmentManager.SetCommitmentType(commitmentType)
	}
}

// WithForkAfter provides after how many slots from the spammer setup should fork bee created, this option can be used with CommitmentType: fork.
func WithForkAfter(forkingAfter int) Options {
	return func(s *Spammer) {
		s.CommitmentManager.SetForkAfter(forkingAfter)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type SpamDetails struct {
	Rate           int
	TimeUnit       time.Duration
	MaxDuration    time.Duration
	MaxBatchesSent int
}

type CommitmentSpamDetails struct {
	CommitmentType string
}
