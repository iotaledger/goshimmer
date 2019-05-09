package ternary

type BCTernaryDemultiplexer struct {
    bcTrinary BCTrinary
}

func NewBCTernaryDemultiplexer(bcTrinary BCTrinary) *BCTernaryDemultiplexer {
    this := &BCTernaryDemultiplexer{bcTrinary: bcTrinary}

    return this
}

func (this *BCTernaryDemultiplexer) Get(index int) Trits {
    length := len(this.bcTrinary.Lo)
    result := make(Trits, length)

    for i := 0; i < length; i++ {
        low := (this.bcTrinary.Lo[i] >> uint(index)) & 1
        hi := (this.bcTrinary.Hi[i] >> uint(index)) & 1

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
