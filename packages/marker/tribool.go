package marker

type TriBool uint8

func (t TriBool) String() string {
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

const (
	False TriBool = iota
	True
	Maybe
)
