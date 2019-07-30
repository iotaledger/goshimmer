package ternary

import (
	"errors"
	"strconv"

	. "github.com/iotaledger/iota.go/trinary"
)

type BCTernaryMultiplexer struct {
	trinaries []Trits
}

func NewBCTernaryMultiplexer() *BCTernaryMultiplexer {
	this := &BCTernaryMultiplexer{make([]Trits, 0)}

	return this
}

func (this *BCTernaryMultiplexer) Add(trits Trits) int {
	this.trinaries = append(this.trinaries, trits)

	return len(this.trinaries) - 1
}

func (this *BCTernaryMultiplexer) Get(index int) Trits {
	return this.trinaries[index]
}

func (this *BCTernaryMultiplexer) Extract() (BCTrits, error) {
	trinariesCount := len(this.trinaries)
	tritsCount := len(this.trinaries[0])

	result := BCTrits{
		Lo: make([]uint, tritsCount),
		Hi: make([]uint, tritsCount),
	}

	for i := 0; i < tritsCount; i++ {
		bcTrit := &BCTrit{0, 0}

		for j := 0; j < trinariesCount; j++ {
			switch this.trinaries[j][i] {
			case -1:
				bcTrit.Lo |= 1 << uint(j)

			case 1:
				bcTrit.Hi |= 1 << uint(j)

			case 0:
				bcTrit.Lo |= 1 << uint(j)
				bcTrit.Hi |= 1 << uint(j)

			default:
				return result, errors.New("Invalid trit #" + strconv.Itoa(i) + " in trits #" + strconv.Itoa(j))
			}
		}

		result.Lo[i] = bcTrit.Lo
		result.Hi[i] = bcTrit.Hi
	}

	return result, nil
}
