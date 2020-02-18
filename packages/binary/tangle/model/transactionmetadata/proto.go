package transactionmetadata

import (
	"fmt"
	"reflect"
	"runtime"
	"time"
	"unsafe"
)

type Proto struct {
	receivedTime       time.Time
	solidificationTime time.Time
	solid              bool
}

// region GENERIC SERIALIZATION CODE ///////////////////////////////////////////////////////////////////////////////////

var sizeOfProto = int(unsafe.Sizeof(Proto{}))

func ProtoFromBytes(bytes []byte) (result *Proto, err error) {
	if bytesLength := len(bytes); bytesLength != sizeOfProto {
		return nil, fmt.Errorf("bytes are not long enough (%d instead of %d)", bytesLength, sizeOfProto)
	}

	copiedBytes := make([]byte, sizeOfProto)
	copy(copiedBytes, bytes)

	result = (*Proto)(unsafe.Pointer(
		(*reflect.SliceHeader)(unsafe.Pointer(&copiedBytes)).Data,
	))

	runtime.KeepAlive(copiedBytes)

	return
}

func (proto *Proto) ToBytes() (result []byte) {
	result = make([]byte, sizeOfProto)
	copy(result, *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(proto)),
		Len:  sizeOfProto,
		Cap:  sizeOfProto,
	})))

	runtime.KeepAlive(proto)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
