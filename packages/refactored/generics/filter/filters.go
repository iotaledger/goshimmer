package filter

func Contains[T comparable](collection map[T]any) func(T) bool {
	return func(candidate T) bool {
		_, exists := collection[candidate]
		return exists
	}
}

func Equal[T](a T) func(b T) bool {
	return func(b T) bool {
		return a == b
	}
}
