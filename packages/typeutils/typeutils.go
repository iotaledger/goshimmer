package typeutils

import (
	"reflect"
	"unsafe"
)

func BytesToString(b []byte) string {
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))

	return *(*string)(unsafe.Pointer(&reflect.StringHeader{bh.Data, bh.Len}))
}

func StringToBytes(str string) []byte {
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&str))

	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: hdr.Data, Len: hdr.Len, Cap: hdr.Len}))
}

func IsInterfaceNil(param interface{}) bool {
	return param == nil || (*[2]uintptr)(unsafe.Pointer(&param))[1] == 0
}
