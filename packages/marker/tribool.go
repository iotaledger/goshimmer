package marker

const (
	// False represents the equivalent of the boolean false value.
	False TriBool = iota

	// True represents the equivalent of the boolean true value.
	True

	// Maybe represents an indeterminate where we are not entirely sure if the value is True or False.
	Maybe
)

// TriBool represents a boolean value that can have an additional Maybe state. It is used to represent the result of
// the past cone check which not always returns a clear answer but sometimes returns a Maybe state that can only be
// determined by walking the underlying DAG.
type TriBool uint8

// String returns a human readable version of the TriBool.
func (t TriBool) String() (humanReadableTriBool string) {
	switch t {
	case 0:
		return "false"
	case 1:
		return "true"
	case 2:
		return "maybe"
	default:
		panic("invalid TriBool value")
	}
}
