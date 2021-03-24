package messagelayer

import "golang.org/x/xerrors"

// ErrQueryNotAllowed is returned when the node is not synced and mana debug mode is disabled.
var ErrQueryNotAllowed = xerrors.New("mana query not allowed, node is not synced, debug mode disabled")
