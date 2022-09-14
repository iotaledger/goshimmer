package storable

// StructConstraint is a constraint that is used to ensure that the given type is a valid Struct.
type StructConstraint[A any, B Pointer[A]] interface {
	*A

	InitStruct(B, string) B
	FromBytes([]byte) (int, error)
	Bytes() ([]byte, error)
}

// Pointer is a constraint that is used to ensure that the given type is a pointer.
type Pointer[A any] interface {
	*A
}
