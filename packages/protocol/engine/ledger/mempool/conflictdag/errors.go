package conflictdag

import "golang.org/x/xerrors"

var (
	ErrExpected       = xerrors.New("expected error")
	ErrEntityEvicted  = xerrors.New("tried to operate on evicted entity")
	ErrFatal          = xerrors.New("fatal error")
	ErrConflictExists = xerrors.New("conflict already exists")
)
