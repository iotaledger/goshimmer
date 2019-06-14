package datastructure

import (
	"time"
)

type LRUCacheOptions struct {
	EvictionCallback func(key interface{}, value interface{})
	IdleTimeout      time.Duration
}

var DEFAULT_OPTIONS = &LRUCacheOptions{
	EvictionCallback: nil,
	IdleTimeout:      30 * time.Second,
}
