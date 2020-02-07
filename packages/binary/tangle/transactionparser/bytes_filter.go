package transactionparser

type BytesFilter interface {
	Filter(bytes []byte)
	OnAccept(callback func(bytes []byte))
	OnReject(callback func(bytes []byte))
	Shutdown()
}
