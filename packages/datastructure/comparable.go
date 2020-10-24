package datastructure

type Comparable interface {
	Compare(other Comparable) int
}
