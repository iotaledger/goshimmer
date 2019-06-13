package datastructure

import (
	"github.com/iotaledger/goshimmer/packages/errors"
)

var (
	ErrNoSuchElement   = errors.New("element does not exist")
	ErrInvalidArgument = errors.New("invalid argument")
)
