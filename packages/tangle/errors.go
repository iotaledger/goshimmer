package tangle

import "errors"

// ErrNotSynced is triggered when somebody tries to issue a Payload before the Tangle is fully synced.
var ErrNotSynced = errors.New("tangle not synced")
