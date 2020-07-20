package messagerequester

import (
	"time"
)

// Options holds options for a message requester.
type Options struct {
	retryInterval time.Duration
}

func newOptions(optionalOptions []Option) *Options {
	result := &Options{
		retryInterval: 10 * time.Second,
	}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}

	return result
}

// Option is a function which inits an option.
type Option func(*Options)

// RetryInterval creates an option which sets the retry interval to the given value.
func RetryInterval(interval time.Duration) Option {
	return func(args *Options) {
		args.retryInterval = interval
	}
}
