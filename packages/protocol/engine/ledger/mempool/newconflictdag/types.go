package newconflictdag

type IDType interface {
	comparable
	Bytes() ([]byte, error)
	String() string
}

const (
	Smaller = -1
	Equal   = 0
	Larger  = 1
)
