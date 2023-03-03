package module

// Provider is a function that returns a module.
type Provider[ContainerType any, ModuleType Interface] func(ContainerType) ModuleType

// Provide turns a constructor into a provider.
func Provide[ContainerType any, ModuleType Interface](constructor func(ContainerType) ModuleType) Provider[ContainerType, ModuleType] {
	return func(c ContainerType) ModuleType {
		return constructor(c)
	}
}
