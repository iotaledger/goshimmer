package slotgadget

import (
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/module"
)

type Gadget interface {
	Events() *Events

	LastConfirmedSlot() slot.Index

	module.Interface
}
