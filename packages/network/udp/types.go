package udp

import "net"

type Callback = func()

type AddressDataConsumer = func(addr *net.UDPAddr, data []byte)

type ErrorConsumer = func(err error)
