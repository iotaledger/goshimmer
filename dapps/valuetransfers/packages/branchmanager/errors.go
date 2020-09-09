package branchmanager

import (
	"errors"
)

var (
	ErrBranchNotFound = errors.New("branch not found in database")
)
