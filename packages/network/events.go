package network

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Events struct {
	BlockReceived                  *event.Linkable[*BlockReceivedEvent]
	BlockRequestReceived           *event.Linkable[*BlockRequestReceivedEvent]
	EpochCommitmentReceived        *event.Linkable[*EpochCommitmentReceivedEvent]
	EpochCommitmentRequestReceived *event.Linkable[*EpochCommitmentRequestReceivedEvent]
	AttestationsReceived           *event.Linkable[*AttestationsReceivedEvent]
	AttestationsRequestReceived    *event.Linkable[*AttestationsRequestReceivedEvent]
	Error                          *event.Linkable[*ErrorEvent]

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		BlockReceived:                  event.NewLinkable[*BlockReceivedEvent](),
		BlockRequestReceived:           event.NewLinkable[*BlockRequestReceivedEvent](),
		EpochCommitmentReceived:        event.NewLinkable[*EpochCommitmentReceivedEvent](),
		EpochCommitmentRequestReceived: event.NewLinkable[*EpochCommitmentRequestReceivedEvent](),
		AttestationsReceived:           event.NewLinkable[*AttestationsReceivedEvent](),
		AttestationsRequestReceived:    event.NewLinkable[*AttestationsRequestReceivedEvent](),
		Error:                          event.NewLinkable[*ErrorEvent](),
	}
})

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockReceivedEvent ///////////////////////////////////////////////////////////////////////////////////////////

type BlockReceivedEvent struct {
	Block *models.Block

	Source identity.ID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockRequestReceivedEvent ////////////////////////////////////////////////////////////////////////////////////

type BlockRequestReceivedEvent struct {
	BlockID models.BlockID

	Source identity.ID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EpochCommitmentReceivedEvent /////////////////////////////////////////////////////////////////////////////////

type EpochCommitmentReceivedEvent struct {
	Commitment *commitment.Commitment
	Source     identity.ID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EpochCommitmentRequestReceivedEvent //////////////////////////////////////////////////////////////////////////

type EpochCommitmentRequestReceivedEvent struct {
	CommitmentID commitment.ID

	Source identity.ID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AttestationsReceivedEvent ////////////////////////////////////////////////////////////////////////////////////

type AttestationsReceivedEvent struct {
	Attestations []*sybilprotection.Attestation

	Source identity.ID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AttestationsRequestReceivedEvent /////////////////////////////////////////////////////////////////////////////

type AttestationsRequestReceivedEvent struct {
	Index epoch.Index

	Source identity.ID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ErrorEvent ///////////////////////////////////////////////////////////////////////////////////////////////////

type ErrorEvent struct {
	Error  error
	Source identity.ID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
