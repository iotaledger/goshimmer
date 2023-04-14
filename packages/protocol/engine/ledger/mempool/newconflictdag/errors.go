package newconflictdag

import "golang.org/x/xerrors"

var (
	ErrEntityEvicted = xerrors.New("tried to operate on evicted entity")
	ErrFatal         = xerrors.New("fatal error")
)
