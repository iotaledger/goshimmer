package metrics

var Events *CollectionEvents

func init() {
	Events = newCollectionEvents()
}
