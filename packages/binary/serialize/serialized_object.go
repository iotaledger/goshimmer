package serialize

import (
	"reflect"
	"unsafe"
)

type SerializedObject struct {
	readOffset int
}

func (so *SerializedObject) SerializeInt(int int) []byte {
	hdr := reflect.SliceHeader{Data: uintptr(unsafe.Pointer(&int)), Len: 8, Cap: 8}
	return *(*[]byte)(unsafe.Pointer(&hdr))
}
