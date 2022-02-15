package evilspammer

import "time"

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

// WithSpamWallet provides initWallet of spammer, if omitted spammer will prepare funds based on maxMsgSent parameter
func WithSpamWallet(initWallets Wallet) Options {
	return func(s *Spammer) {
		s.InputFunds = initWallets
	}
}

// WithClients sets clients that will be used for spam, cannot be omitted, no default options.
func WithClients(clients Clients) Options {
	return func(s *Spammer) {
		s.Clients = clients
	}
}

// WithErrorCounter allows for setting an error counter object, if not provided a new instance will be created.
func WithErrorCounter(errCounter ErrorCounter) Options {
	return func(s *Spammer) {
		s.errCounter = errCounter
	}
}

// WithLogTickerInterval allows for changing interval between progress spamming logs, default is 30s.
func WithLogTickerInterval(interval time.Duration) Options {
	return func(s *Spammer) {
		s.spammerState.logTickTime = interval
	}
}

// WithSpammingFunc sets core function of the spammer with spamming logic, needs to use done spammer's channel to communicate.
// end of spamming and errors.
func WithSpammingFunc(spammerFunc func(s *Spammer)) Options {
	return func(s *Spammer) {
		s.spamFunc = spammerFunc
	}
}

// WithOutputWallets allows to set the output wallets for the spammer, that will gather outputs created during the spam for the future usage
// SpammingFunc provided to the spammer needs to support this functionality.
func WithOutputWallets(w Wallet) Options {
	return func(s *Spammer) {
		s.OutputFunds = w
	}
}

func WithTimeDelayForDoubleSpend(timeDelay time.Duration) Options {
	return func(s *Spammer) {
		s.TimeDelayBetweenConflicts = timeDelay
	}
}

// WithIssueAPIMethod sets issuing API method for the spammer, default is PostTransaction.
func WithIssueAPIMethod(methodName string) Options {
	return func(s *Spammer) {
		switch methodName {
		case "SendPayload":
			s.IssuingAPIMethod = "SendPayload"
		default:
			s.IssuingAPIMethod = "PostTransaction"
		}
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
