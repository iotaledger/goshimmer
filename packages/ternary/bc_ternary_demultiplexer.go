package ternary

import (
	. "github.com/iotaledger/iota.go/trinary"
)

type BCTernaryDemultiplexer struct {
	bcTrits BCTrits
}

func NewBCTernaryDemultiplexer(bcTrits BCTrits) *BCTernaryDemultiplexer {
	this := &BCTernaryDemultiplexer{bcTrits: bcTrits}

	return this
}

func (this *BCTernaryDemultiplexer) Get(index int) Trits {
	length := len(this.bcTrits.Lo)
	result := make(Trits, length)

	for i := 0; i < length; i++ {
		low := (this.bcTrits.Lo[i] >> uint(index)) & 1
		hi := (this.bcTrits.Hi[i] >> uint(index)) & 1

		switch true {
		case low == 1 && hi == 0:
			result[i] = -1

		case low == 0 && hi == 1:
			result[i] = 1

		case low == 1 && hi == 1:
			result[i] = 0

		default:
			result[i] = 0
		}
	}

	return result
}
