package causalorder

type OrderStatusUpdate uint8

const (
	Unchanged OrderStatusUpdate = iota
	Ordered
	Invalid
)
