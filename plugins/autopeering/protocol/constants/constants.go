package constants

import "time"

const (
    NEIGHBOR_COUNT = 8

    FIND_NEIGHBOR_INTERVAL     = 5 * time.Second
    PING_RANDOM_PEERS_INTERVAL = 1 * time.Second
    PING_RANDOM_PEERS_COUNT    = 3
)
