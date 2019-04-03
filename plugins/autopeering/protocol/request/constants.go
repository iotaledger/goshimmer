package request

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/salt"
)

const (
    PACKET_HEADER_SIZE = 1
    ISSUER_SIZE        = peer.MARSHALLED_TOTAL_SIZE
    SALT_SIZE          = salt.SALT_MARSHALLED_SIZE
    SIGNATURE_SIZE     = 65

    PACKET_HEADER_START = 0
    ISSUER_START        = PACKET_HEADER_END
    SALT_START          = ISSUER_END
    SIGNATURE_START     = SALT_END

    PACKET_HEADER_END = PACKET_HEADER_START + PACKET_HEADER_SIZE
    ISSUER_END        = ISSUER_START + ISSUER_SIZE
    SALT_END          = SALT_START + SALT_SIZE
    SIGNATURE_END     = SIGNATURE_START + SIGNATURE_SIZE

    MARSHALLED_TOTAL_SIZE = SIGNATURE_END

    MARSHALLED_PACKET_HEADER = 0xBE
)
