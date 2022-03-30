package utxo

type Iterator[T any] struct {
	elements     []T
	currentIndex int
	stopped      bool
}

func NewIterator[T any](elements ...T) *Iterator[T] {
	return &Iterator[T]{
		elements: elements,
	}
}

func (i *Iterator[T]) HasNext() bool {
	return !i.stopped && i.currentIndex < len(i.elements)
}

func (i *Iterator[T]) Next() (next T) {
	next = i.elements[i.currentIndex]
	i.currentIndex++
	return next
}

func (i *Iterator[T]) Stop() {
	i.stopped = true
}

func (i *Iterator[T]) Reset() {
	i.stopped = false
	i.currentIndex = 0
}
