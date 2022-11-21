package utils

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Events struct {
	BlockDiscarded *event.Event[*models.Block]
	BlockIssued    *event.Event[*models.Block]
	Error          *event.Event[error]
}

func NewEvents() (new *Events) {
	return &Events{
		BlockDiscarded: event.New[*models.Block](),
		BlockIssued:    event.New[*models.Block](),
		Error:          event.New[error](),
	}
}
