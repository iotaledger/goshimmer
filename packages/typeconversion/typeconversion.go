package typeconversion

import (
    "reflect"
    "unsafe"
)

func BytesToString(b []byte) string {
    bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))

    return *(*string)(unsafe.Pointer(&reflect.StringHeader{bh.Data, bh.Len}))
}