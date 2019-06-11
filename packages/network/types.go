package network

type Callback func()

type ErrorConsumer func(err error)

type DataConsumer func(data []byte)
