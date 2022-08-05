package tangleold

import (
	"github.com/iotaledger/goshimmer/packages/core/consensus"
)

// region OTVConsensusManager /////////////////////////////////////////////////////////////////////////////////////////////

// OTVConsensusManager is the component in charge of forming opinions about conflicts.
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
