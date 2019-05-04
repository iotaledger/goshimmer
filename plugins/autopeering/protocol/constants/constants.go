package constants

import "time"

const (
    NEIGHBOR_COUNT = 8

    FIND_NEIGHBOR_INTERVAL = 10 * time.Second

    // How often does the outgoing ping processor check if new pings should be sent.
    PING_PROCESS_INTERVAL = 1 * time.Second

    // The amount of times each neighbor should be contacted in this cycle.
    PING_CONTACT_COUNT_PER_CYCLE = 2

    // The length of a ping cycle (after this time we have sent randomized pings to all of our neighbors).
    PING_CYCLE_LENGTH = 900 * time.Second
)
