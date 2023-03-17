package conflict

// IDType is the constraint for the identifier of a conflict or a resource.
type IDType interface {
	// comparable is a built-in constraint that ensures that the type can be used as a map key.
	comparable

	// Bytes returns a serialized version of the ID.
	Bytes() ([]byte, error)

	// String returns a human-readable version of the ID.
	String() string
}
