package transactionrequester

import (
	"time"
)

type Options struct {
	retryInterval time.Duration
	workerCount   int
}

func newOptions(optionalOptions []Option) *Options {
	result := &Options{
		retryInterval: 10 * time.Second,
		workerCount:   DEFAULT_REQUEST_WORKER_COUNT,
	}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}

	return result
}

type Option func(*Options)

func RetryInterval(interval time.Duration) Option {
	return func(args *Options) {
		args.retryInterval = interval
	}
}

func WorkerCount(workerCount int) Option {
	return func(args *Options) {
		args.workerCount = workerCount
	}
}
