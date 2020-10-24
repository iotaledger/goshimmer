package datastructure

type Range_ee interface {
	Beginning() Key

	End() Key
}

type UInt64Range struct {
	beginning Key
	end       Key
}

func (U UInt64Range) Beginning() Key {
	return U.beginning
}

func (U UInt64Range) End() Key {
	return U.end
}

var _ Range_ee = UInt64Range{}
