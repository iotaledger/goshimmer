package newconflictdag

type IDType interface {
	comparable
	Bytes() ([]byte, error)
	String() string
}
