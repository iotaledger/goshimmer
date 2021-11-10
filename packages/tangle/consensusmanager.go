package tangle

import (
	"github.com/iotaledger/goshimmer/packages/consensus"
)

// region OTVConsensusManager /////////////////////////////////////////////////////////////////////////////////////////////

// OTVConsensusManager is the component in charge of forming opinions about branches.
type OTVConsensusManager struct {
	consensus.Mechanism
}

// NewOTVConsensusManager returns a new Mechanism.
func NewOTVConsensusManager(otvConsensusMechanism consensus.Mechanism) *OTVConsensusManager {
	return &OTVConsensusManager{
		Mechanism: otvConsensusMechanism,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
