package engine

type (
	ModuleProvider[ModuleType any] func(engine *Engine) ModuleType
)

func ProvideModule[ModuleType any](constructor func(engine *Engine) ModuleType) ModuleProvider[ModuleType] {
	return func(e *Engine) ModuleType {
		return constructor(e)
	}
}
