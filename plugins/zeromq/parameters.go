package zeromq

import "github.com/iotaledger/goshimmer/packages/parameter"

var (
	PORT = parameter.AddInt("ZEROMQ/PORT", 5556, "tcp port used to connect to the zmq feed")
)
