package remotemetrics

var Events *CollectionLogEvents

func init() {
	Events = newCollectionLogEvents()
}
