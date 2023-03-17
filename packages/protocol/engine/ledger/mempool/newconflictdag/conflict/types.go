package conflict

// IDType is the interface that defines the constraints for the ID of a conflict or a resource.
type IDType interface {
	// comparable is a built-in interface implemented by types that can be compared using the == operator.
	comparable

	// Bytes returns a serialized version of the ID.
	Bytes() ([]byte, error)

	// String returns a human-readable version of the ID.
	String() string
}
