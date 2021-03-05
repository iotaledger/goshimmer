package mana

import "golang.org/x/xerrors"

var (
	// ErrQueryNotAllowed is returned when the node is not synced and mana debug mode is disabled.
	ErrQueryNotAllowed = xerrors.New("mana query not allowed, node is not synced, debug mode disabled")
)
