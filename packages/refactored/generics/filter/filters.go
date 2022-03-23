package filter

func MapHasKey[T comparable, V any](collection map[T]V) func(T) bool {
	return func(candidate T) bool {
		_, exists := collection[candidate]
		return exists
	}
}

func Equal[T comparable](a T) func(b T) bool {
	return func(b T) bool {
		return a == b
	}
}
