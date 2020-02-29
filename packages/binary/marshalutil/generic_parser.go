package marshalutil

type GenericParser func(data []byte) (result interface{}, err error, consumedBytes int)
