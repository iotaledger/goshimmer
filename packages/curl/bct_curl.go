package curl

import "github.com/iotaledger/goshimmer/packages/ternary"

const (
	HIGH_LONG_BITS             = 0xFFFFFFFFFFFFFFFF
	NUMBER_OF_TRITS_IN_A_TRYTE = 3
)

type BCTCurl struct {
	hashLength     int
	numberOfRounds int
	stateLength    int
	state          ternary.BCTrits
	cTransform     func()
}

func NewBCTCurl(hashLength int, numberOfRounds int) *BCTCurl {
	this := &BCTCurl{
		hashLength:     hashLength,
		numberOfRounds: numberOfRounds,
		stateLength:    NUMBER_OF_TRITS_IN_A_TRYTE * hashLength,
		state: ternary.BCTrits{
			Lo: make([]uint, NUMBER_OF_TRITS_IN_A_TRYTE*hashLength),
			Hi: make([]uint, NUMBER_OF_TRITS_IN_A_TRYTE*hashLength),
		},
		cTransform: nil,
	}

	this.Reset()

	return this
}

func (this *BCTCurl) Reset() {
	for i := 0; i < this.stateLength; i++ {
		this.state.Lo[i] = HIGH_LONG_BITS
		this.state.Hi[i] = HIGH_LONG_BITS
	}
}

func (this *BCTCurl) Transform() {
	scratchPadLo := make([]uint, this.stateLength)
	scratchPadHi := make([]uint, this.stateLength)
	scratchPadIndex := 0

	for round := this.numberOfRounds; round > 0; round-- {
		copy(scratchPadLo, this.state.Lo)
		copy(scratchPadHi, this.state.Hi)
		for stateIndex := 0; stateIndex < this.stateLength; stateIndex++ {
			alpha := scratchPadLo[scratchPadIndex]
			beta := scratchPadHi[scratchPadIndex]

			if scratchPadIndex < 365 {
				scratchPadIndex += 364
			} else {
				scratchPadIndex -= 365
			}

			delta := beta ^ scratchPadLo[scratchPadIndex]

			this.state.Lo[stateIndex] = ^(delta & alpha)
			this.state.Hi[stateIndex] = delta | (alpha ^ scratchPadHi[scratchPadIndex])
		}
	}
}

func (this *BCTCurl) Absorb(bcTrits ternary.BCTrits) {
	length := len(bcTrits.Lo)
	offset := 0

	for {
		var lengthToCopy int
		if length < this.hashLength {
			lengthToCopy = length
		} else {
			lengthToCopy = this.hashLength
		}

		copy(this.state.Lo[0:lengthToCopy], bcTrits.Lo[offset:offset+lengthToCopy])
		copy(this.state.Hi[0:lengthToCopy], bcTrits.Hi[offset:offset+lengthToCopy])
		this.Transform()

		offset += lengthToCopy
		length -= lengthToCopy

		if length <= 0 {
			break
		}
	}
}

func (this *BCTCurl) Squeeze(tritCount int) ternary.BCTrits {
	result := ternary.BCTrits{
		Lo: make([]uint, tritCount),
		Hi: make([]uint, tritCount),
	}
	hashCount := tritCount / this.hashLength

	for i := 0; i < hashCount; i++ {
		copy(result.Lo[i*this.hashLength:(i+1)*this.hashLength], this.state.Lo[0:this.hashLength])
		copy(result.Hi[i*this.hashLength:(i+1)*this.hashLength], this.state.Hi[0:this.hashLength])

		this.Transform()
	}

	last := tritCount - hashCount*this.hashLength

	copy(result.Lo[tritCount-last:], this.state.Lo[0:last])
	copy(result.Hi[tritCount-last:], this.state.Hi[0:last])

	if tritCount%this.hashLength != 0 {
		this.Transform()
	}

	return result
}
