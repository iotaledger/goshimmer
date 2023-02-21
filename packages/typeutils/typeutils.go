package typeutils

import (
	"unsafe"
)

// IsInterfaceNil Checks whether an interface is nil or has the value nil.
func IsInterfaceNil(param interface{}) bool {
	return param == nil || (*[2]uintptr)(unsafe.Pointer(&param))[1] == 0
}
