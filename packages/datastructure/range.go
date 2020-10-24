package datastructure

// A Range defines the boundaries around a contiguous span of values of some Comparable type (i.e. "integers from 1 to
// 100 inclusive").
//
// It is not possible to iterate over the contained values. Each Range may be bounded or unbounded. If bounded, there is
// an associated endpoint value, and the range is considered to be either open (does not include the endpoint) or closed
// (includes the endpoint) on that side.
//
// With three possibilities on each side, this yields nine basic types of ranges, enumerated below:
//
// Notation         Definition          Factory method
// (a .. b)         {x | a < x < b}     Open
// [a .. b]         {x | a <= x <= b}   Closed
// (a .. b]         {x | a < x <= b}    OpenClosed
// [a .. b)         {x | a <= x < b}    ClosedOpen
// (a .. +INF)      {x | x > a}         GreaterThan
// [a .. +INF)      {x | x >= a}        AtLeast
// (-INF .. b)      {x | x < b}         LessThan
// (-INF .. b]      {x | x <= b}        AtMost
// (-INF .. +INF)   {x}        			All
//
// When both endpoints exist, the upper endpoint may not be less than the lower. The endpoints may be equal only if at
// least one of the bounds is closed.
type Range struct {
	lowerEndPoint *EndPoint
	upperEndPoint *EndPoint
}

// HasUpperBound returns true if this Range has an upper EndPoint.
func (r *Range) HasUpperBound() bool {
	return r.upperEndPoint != nil
}

// UpperBoundType returns the type of this Range's upper bound - BoundTypeClosed if the range includes its upper
// EndPoint and BoundTypeOpen if it is either unbounded or does not include its upper EndPoint.
func (r *Range) UpperBoundType() BoundType {
	if r.upperEndPoint == nil {
		return BoundTypeOpen
	}

	return r.upperEndPoint.boundType
}

// UpperEndPoint returns the upper EndPoint of this Range or nil if it is unbounded.
func (r *Range) UpperEndPoint() *EndPoint {
	return r.upperEndPoint
}
