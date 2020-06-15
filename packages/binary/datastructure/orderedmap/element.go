package orderedmap

// Element defines the model of each element of the orderedMap.
type Element struct {
	key   interface{}
	value interface{}
	prev  *Element
	next  *Element
}
