package filter

import "testing"

func BenchmarkAdd(b *testing.B) {
	filter, byteArray := setupFilter(15000, 1604)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		filter.Add(byteArray)
	}
}

func BenchmarkContains(b *testing.B) {
	filter, byteArray := setupFilter(15000, 1604)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		filter.Contains(byteArray)
	}
}

func setupFilter(filterSize int, byteArraySize int) (*ByteArrayFilter, []byte) {
	filter := NewByteArrayFilter(filterSize)

	for j := 0; j < filterSize; j++ {
		byteArray := make([]byte, byteArraySize)

		for i := 0; i < len(byteArray); i++ {
			byteArray[(i+j)%byteArraySize] = byte((i + j) % 128)
		}

		filter.Add(byteArray)
	}

	byteArray := make([]byte, byteArraySize)

	for i := 0; i < len(byteArray); i++ {
		byteArray[i] = byte(i % 128)
	}

	return filter, byteArray
}
