package filter

import (
	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/crypto/identity"
)

type Filter interface {
	Events() *Events

	// ProcessReceivedBlock processes block from the given source.
	ProcessReceivedBlock(block *models.Block, source identity.ID)

	module.Interface
}
