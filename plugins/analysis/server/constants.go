package server

import (
    "github.com/iotaledger/goshimmer/plugins/analysis/types/addnode"
    "github.com/iotaledger/goshimmer/plugins/analysis/types/connectnodes"
    "github.com/iotaledger/goshimmer/plugins/analysis/types/disconnectnodes"
    "github.com/iotaledger/goshimmer/plugins/analysis/types/ping"
    "github.com/iotaledger/goshimmer/plugins/analysis/types/removenode"
    "time"
)

const (
    IDLE_TIMEOUT = 5 * time.Second

    STATE_INITIAL          = byte(255)
    STATE_INITIAL_ADDNODE  = byte(254)
    STATE_CONSECUTIVE      = byte(253)
    STATE_PING             = ping.MARSHALLED_PACKET_HEADER
    STATE_ADD_NODE         = addnode.MARSHALLED_PACKET_HEADER
    STATE_REMOVE_NODE      = removenode.MARSHALLED_PACKET_HEADER
    STATE_CONNECT_NODES    = connectnodes.MARSHALLED_PACKET_HEADER
    STATE_DISCONNECT_NODES = disconnectnodes.MARSHALLED_PACKET_HEADER
)
