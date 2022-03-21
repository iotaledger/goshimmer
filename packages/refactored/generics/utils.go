package generics

func Map[SourceType any, TargetType any](source []SourceType, mapper func(SourceType) TargetType) (target []TargetType) {
	target = make([]TargetType, len(source))
	for i, value := range source {
		target[i] = mapper(value)
	}

	return target
}

// Reduce reduces collection to a value which is the accumulated result of running each element in collection
// through accumulator, where each successive invocation is supplied the return value of the previous.
func Reduce[T any, R any](collection []T, accumulator func(R, T) R, initial R) R {
	for _, item := range collection {
		initial = accumulator(initial, item)
	}

	return initial
}

// Filter iterates over elements of collection, returning an array of all elements predicate returns truthy for.
func Filter[V any](collection []V, predicate func(V) bool) []V {
	result := []V{}

	for _, item := range collection {
		if predicate(item) {
			result = append(result, item)
		}
	}

	return result
}

// KeyBy transforms a slice or an array of structs to a map based on a pivot callback.
func KeyBy[K comparable, V any](collection []V, iteratee func(V) K) map[K]V {
	result := make(map[K]V, len(collection))

	for _, v := range collection {
		k := iteratee(v)
		result[k] = v
	}

	return result
}

// FilterByValue iterates over the map, returning a map of all elements predicate returns truthy for.
func FilterByValue[K comparable, V any](collection map[K]V, predicate func(V) bool) map[K]V {
	result := make(map[K]V)
	for key, value := range collection {
		if predicate(value) {
			result[key] = value
		}
	}

	return result
}

// Keys creates an array of the map keys.
func Keys[K comparable, V any](in map[K]V) []K {
	result := make([]K, 0, len(in))

	for k, _ := range in {
		result = append(result, k)
	}

	return result
}

// Values creates an array of the map values.
func Values[K comparable, V any](in map[K]V) []V {
	result := make([]V, 0, len(in))

	for _, v := range in {
		result = append(result, v)
	}

	return result
}

// ForEach iterates over elements of collection and invokes iteratee for each element.
func ForEach[T any](collection []T, iteratee func(T)) {
	for _, item := range collection {
		iteratee(item)
	}
}

// ForEach iterates over elements of collection and invokes iteratee for each element.
func ReduceProperty[A, B, C any](collection []A, propertyResolver func(A) B, accumulator func(C, B) C, initial C) C {
	for _, item := range collection {
		initial = accumulator(initial, propertyResolver(item))
	}

	return initial
}

func Bind[FirstParamType, ParamType, ReturnType any](secondParam ParamType, callback func(FirstParamType, ParamType) ReturnType) func(FirstParamType) ReturnType {
	return func(firstParam FirstParamType) ReturnType {
		return callback(firstParam, secondParam)
	}
}

func Identity[T any](param T) T {
	return param
}
