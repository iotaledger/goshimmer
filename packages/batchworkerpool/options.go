package batchworkerpool

import (
	"runtime"
	"time"
)

var DEFAULT_OPTIONS = &Options{
	WorkerCount:            2 * runtime.NumCPU(),
	QueueSize:              2 * runtime.NumCPU() * 64,
	BatchSize:              64,
	BatchCollectionTimeout: 15 * time.Millisecond,
}

func WorkerCount(workerCount int) Option {
	return func(args *Options) {
		args.WorkerCount = workerCount
	}
}

func BatchSize(batchSize int) Option {
	return func(args *Options) {
		args.BatchSize = batchSize
	}
}

func BatchCollectionTimeout(batchCollectionTimeout time.Duration) Option {
	return func(args *Options) {
		args.BatchCollectionTimeout = batchCollectionTimeout
	}
}

func QueueSize(queueSize int) Option {
	return func(args *Options) {
		args.QueueSize = queueSize
	}
}

type Options struct {
	WorkerCount            int
	QueueSize              int
	BatchSize              int
	BatchCollectionTimeout time.Duration
}

func (options Options) Override(optionalOptions ...Option) *Options {
	result := &options
	for _, option := range optionalOptions {
		option(result)
	}

	return result
}

type Option func(*Options)
