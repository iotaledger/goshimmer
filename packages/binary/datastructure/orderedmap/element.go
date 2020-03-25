package orderedmap

type Element struct {
	key   interface{}
	value interface{}
	prev  *Element
	next  *Element
}
