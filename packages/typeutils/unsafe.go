package typeutils

import (
	"unsafe"
)

// BytesToString Converts a slice of bytes into a string without performing a copy.
// NOTE: This is an unsafe operation and may lead to problems if the bytes
// passed as argument are changed while the string is used.  No checking whether
// bytes are valid UTF-8 data is performed.
func BytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// StringToBytes Converts a string into a slice of bytes without performing a copy.
// NOTE: This is an unsafe operation and may lead to problems if the bytes are changed.
func StringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
