package branchdag

type Collection[T any] struct {
	elements []T
}

func (c *Collection[T]) ForEach(callback func(element T)) {
	for _, element := range c.elements {
		callback(element)
	}
}

type TransactionID [32]byte

type TransactionIDs = Collection[TransactionID]

func init() {
	new(TransactionIDs).ForEach(func(element TransactionID) {

	})
}
