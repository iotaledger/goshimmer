package unsafeconvert

import (
	"unsafe"
)

// Converts a slice of bytes into a string without performing a copy.
// NOTE: This is an unsafe operation and may lead to problems if the bytes
// passed as argument are changed while the string is used.  No checking whether
// bytes are valid UTF-8 data is performed.
func BytesToString(bs []byte) string {
	return *(*string)(unsafe.Pointer(&bs))
}

// Converts a string into a slice of bytes without performing a copy.
// NOTE: This is an unsafe operation and may lead to problems if the bytes are changed.
func StringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&s))
}
