package ternary

import (
	"reflect"
	"unsafe"
)

// a Trit can have the values 0, 1 and -1
type Trit = int8

// Trits consists out of many Trits
type Trits []Trit

// Trytes is a string representation of the Trits
type Trytes string

// simply changes the type of this Trytes to a byte array without copying any data
func (trytes Trytes) CastToBytes() []byte {
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&trytes))

	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: hdr.Data, Len: hdr.Len, Cap: hdr.Len}))
}

func (trytes Trytes) ToTrits() Trits {
	trits := make(Trits, 0, len(trytes)*NUMBER_OF_TRITS_IN_A_TRYTE)
	for _, char := range trytes {
		trits = append(trits, TRYTES_TO_TRITS_MAP[char]...)
	}

	return trits
}

func (trytes Trytes) ToString() string {
	return string(trytes)
}

func (this Trits) ToBytes() []byte {
	tritsLength := len(this)
	bytesLength := (tritsLength + NUMBER_OF_TRITS_IN_A_BYTE - 1) / NUMBER_OF_TRITS_IN_A_BYTE

	bytes := make([]byte, bytesLength)
	radix := int8(3)

	tritIdx := bytesLength * NUMBER_OF_TRITS_IN_A_BYTE
	for byteNum := bytesLength - 1; byteNum >= 0; byteNum-- {
		var value int8 = 0

		for i := 0; i < NUMBER_OF_TRITS_IN_A_BYTE; i++ {
			tritIdx--

			if tritIdx < tritsLength {
				value = value*radix + this[tritIdx]
			}
		}
		bytes[byteNum] = byte(value)
	}

	return bytes
}

func (this Trits) TrailingZeroes() int {
	zeros := 0
	index := len(this) - 1
	for index >= 0 && this[index] == 0 {
		zeros++

		index--
	}

	return zeros
}

func (this Trits) ToUint() (result uint) {
	for i := len(this) - 1; i >= 0; i-- {
		result = result*3 + uint(this[i])
	}

	return
}

func (this Trits) ToInt64() int64 {
	var val int64
	for i := len(this) - 1; i >= 0; i-- {
		val = val*3 + int64(this[i])
	}

	return val
}

func (this Trits) ToUint64() uint64 {
	var val uint64
	for i := len(this) - 1; i >= 0; i-- {
		val = val*3 + uint64(this[i])
	}

	return val
}

func (this Trits) ToString() string {
	return TritsToString(this, 0, len(this))
}

func (this Trits) ToTrytes() Trytes {
	return Trytes(TritsToString(this, 0, len(this)))
}
