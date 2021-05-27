package messagelayer

import "github.com/cockroachdb/errors"

// ErrQueryNotAllowed is returned when the node is not synced and mana debug mode is disabled.
var ErrQueryNotAllowed = errors.New("mana query not allowed, node is not synced, debug mode disabled")
