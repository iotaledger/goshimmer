package bitutils

type BitMask byte

func (bitmask BitMask) SetFlag(pos uint) BitMask {
    return bitmask | (1 << pos)
}

func (bitmask BitMask) ClearFlag(pos uint) BitMask {
    return bitmask & ^(1 << pos)
}

func (bitmask BitMask) HasFlag(pos uint) bool {
    return (bitmask & (1 << pos) > 0)
}