package peermanager

import (
    "github.com/iotaledger/goshimmer/packages/identity"
    "time"
)

const (
    FIND_NEIGHBOR_INTERVAL = 5 * time.Second
)

var UNKNOWN_IDENTITY = identity.GenerateRandomIdentity()
