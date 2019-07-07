package unsafeconvert

import (
	"bytes"
	"strings"
	"testing"
)

var testStrings = []string{
	"",
	" ",
	"test",
	"こんにちは、 世界",
	strings.Repeat(" ", 10),
	strings.Repeat(" ", 100),
	strings.Repeat(" ", 10000),
	strings.Repeat(" ", 1000000),
}

func TestBytesToString(t *testing.T) {
	for _, expected := range testStrings {
		arg := []byte(expected)
		actual := BytesToString(arg)
		if actual != expected {
			t.Errorf("BytesToString(%q) = %q but expected %q", arg, actual, expected)
		}
	}
}

func TestStringToBytes(t *testing.T) {
	for _, arg := range testStrings {
		expected := []byte(arg)
		actual := StringToBytes(arg)
		if !bytes.Equal(actual, expected) {
			t.Errorf("Bytes(%q) = %q but expected %q", arg, actual, expected)
		}
	}
}

func TestNil(t *testing.T) {
	actual := BytesToString(nil)
	expected := ""
	if actual != expected {
		t.Errorf("String(nil) = %q but expected %q", actual, expected)
	}
}

func createTestBytes() [][]byte {
	result := make([][]byte, len(testStrings))
	for i, str := range testStrings {
		result[i] = []byte(str)
	}
	return result
}

func BenchmarkNativeBytesToString(b *testing.B) {
	testBytes := createTestBytes()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, bs := range testBytes {
			_ = string(bs)
		}
	}
}

func BenchmarkUnsafeBytesToString(b *testing.B) {
	testBytes := createTestBytes()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, bs := range testBytes {
			_ = BytesToString(bs)
		}
	}
}

func BenchmarkNativeStringToBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, str := range testStrings {
			_ = []byte(str)
		}
	}
}

func BenchmarkUnsafeStringToBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, str := range testStrings {
			_ = StringToBytes(str)
		}
	}
}
