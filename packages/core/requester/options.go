package requester

import (
	"time"

	"github.com/iotaledger/hive.go/generics/options"
)

// RetryInterval creates an option which sets the retry interval to the given value.
func RetryInterval(interval time.Duration) options.Option[Requester] {
	return func(requester *Requester) {
		requester.optsRetryInterval = interval
	}
}

// RetryJitter creates an option which sets the retry jitter to the given value.
func RetryJitter(retryJitter time.Duration) options.Option[Requester] {
	return func(requester *Requester) {
		requester.optsRetryJitter = retryJitter
	}
}

// MaxRequestThreshold creates an option which defines how often the Requester should try to request blocks before
// canceling the request.
func MaxRequestThreshold(maxRequestThreshold int) options.Option[Requester] {
	return func(requester *Requester) {
		requester.optsMaxRequestThreshold = maxRequestThreshold
	}
}
