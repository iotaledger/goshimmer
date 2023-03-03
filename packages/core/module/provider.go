package module

type (
	Provider[ContainerType any, ModuleType Interface] func(ContainerType) ModuleType
)

func Provide[ContainerType any, ModuleType Interface](constructor func(ContainerType) ModuleType) Provider[ContainerType, ModuleType] {
	return func(c ContainerType) ModuleType {
		return constructor(c)
	}
}
