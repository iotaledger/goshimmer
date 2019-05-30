package typeconversion

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

    return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: hdr.Data, Len:  hdr.Len, Cap:  hdr.Len}))
}