package datastructure

// Key represents a value that can be used as a key in maps and associative arrays.
type Key interface {
	// Compare returns 1 if the value is bigger, -1 if the value is smaller and 0 if the values are the same.
	Compare(other Key) int
}
