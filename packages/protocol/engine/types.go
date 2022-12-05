package engine

import (
	"sync"
)

type (
	ModuleProvider[ModuleType any] func(engine *Engine) ModuleType
)

func ProvideModule[ModuleType any](constructor func(engine *Engine) ModuleType) ModuleProvider[ModuleType] {
	var (
		module ModuleType
		once   sync.Once
	)

	return func(e *Engine) ModuleType {
		once.Do(func() { module = constructor(e) })

		return module
	}
}
