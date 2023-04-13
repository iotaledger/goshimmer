package newconflictdag

import "golang.org/x/xerrors"

var (
	EvictionError = xerrors.New("tried to operate on evicted entity")
	RuntimeError  = xerrors.New("unexpected operation")
)
