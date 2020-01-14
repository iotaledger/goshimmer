package tangle

import (
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/trinary"
)

// region plugin module setup //////////////////////////////////////////////////////////////////////////////////////////

var PLUGIN = node.NewPlugin("Tangle", node.Enabled, configure, run)
var log *logger.Logger

func configure(*node.Plugin) {
	log = logger.NewLogger("Tangle")

	configureTransactionDatabase()
	configureTransactionMetaDataDatabase()
	configureApproversDatabase()
	configureBundleDatabase()
	configureTransactionHashesForAddressDatabase()
	configureSolidifier()
}

func run(*node.Plugin) {
	runSolidifier()
}

// Requester provides the functionality to request a transaction from the network.
type Requester interface {
	RequestTransaction(hash trinary.Hash)
}

type RequesterFunc func(hash trinary.Hash)

func (f RequesterFunc) RequestTransaction(hash trinary.Hash) { f(hash) }

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
