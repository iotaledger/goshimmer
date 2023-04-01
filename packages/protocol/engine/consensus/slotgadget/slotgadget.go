package slotgadget

import (
	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/hive.go/core/slot"
)

type Gadget interface {
	Events() *Events

	LastConfirmedSlot() slot.Index

	module.Interface
}
