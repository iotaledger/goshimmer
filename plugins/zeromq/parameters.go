package zeromq

import (
	flag "github.com/spf13/pflag"
)

const (
	ZEROMQ_PORT = "zeromq.port"
)

func init() {
	flag.Int(ZEROMQ_PORT, 5556, "tcp port used to connect to the zmq feed")
}
