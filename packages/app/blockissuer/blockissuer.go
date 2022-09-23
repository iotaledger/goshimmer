package blockissuer

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/app/blockfactory"
	"github.com/iotaledger/goshimmer/packages/app/ratesetter"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/mana/manamodels"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
)

// region Factory ///////////////////////////////////////////////////////////////////////////////////////////////

// BlockIssuer contains logic to create and issue blocks.
type BlockIssuer struct {
	Events *Events

	*blockfactory.Factory
	*ratesetter.RateSetter
	protocol          *protocol.Protocol
	identity          *identity.LocalIdentity
	referenceProvider *blockfactory.ReferenceProvider

	optsRateSetterOptions   []options.Option[ratesetter.RateSetter]
	optsBlockFactoryOptions []options.Option[blockfactory.Factory]
}

// New creates a new block issuer.
func New(protocol *protocol.Protocol, localIdentity *identity.LocalIdentity, opts ...options.Option[BlockIssuer]) *BlockIssuer {
	return options.Apply(&BlockIssuer{
		Events:            NewEvents(),
		identity:          localIdentity,
		protocol:          protocol,
		referenceProvider: blockfactory.NewReferenceProvider(protocol.Instance().Engine, protocol.Instance().NotarizationManager.LatestCommitableEpochIndex),
	}, opts, func(i *BlockIssuer) {
		i.Factory = blockfactory.NewBlockFactory(localIdentity, func(blockID models.BlockID) (block *blockdag.Block, exists bool) {
			return i.protocol.Instance().Engine.Tangle.BlockDAG.Block(blockID)
		}, func(countParents int) (parents models.BlockIDs) {
			return i.protocol.Instance().TipManager.Tips(countParents)
		}, i.referenceProvider.References, func() (ecRecord *commitment.Commitment, lastConfirmedEpochIndex epoch.Index, err error) {
			latestCommitment, err := i.protocol.Instance().NotarizationManager.GetLatestEC()
			if err != nil {
				return nil, 0, err
			}
			confirmedEpochIndex, err := i.protocol.Instance().NotarizationManager.LatestConfirmedEpochIndex()
			if err != nil {
				return nil, 0, err
			}

			return latestCommitment.Commitment(), confirmedEpochIndex, nil
		}, i.optsBlockFactoryOptions...)

		i.RateSetter = ratesetter.New(i.protocol, func() map[identity.ID]int64 {
			manaMap, _, err := i.protocol.Instance().Engine.CongestionControl.Tracker.GetManaMap(manamodels.AccessMana)
			if err != nil {
				return make(map[identity.ID]int64)
			}
			return manaMap
		}, func() int64 {
			totalMana, _, err := i.protocol.Instance().Engine.CongestionControl.Tracker.GetTotalMana(manamodels.AccessMana)
			if err != nil {
				return 0
			}
			return totalMana
		}, i.identity.ID(), i.optsRateSetterOptions...)

	})
}

func (f *BlockIssuer) setupEvents() {
	f.RateSetter.Events.BlockIssued.Attach(event.NewClosure[*models.Block](func(block *models.Block) {
		err := f.RateSetter.Issue(block)
		if err != nil {
			return
		}
	}))
}

// IssuePayload creates a new block including sequence number and tip selection and returns it.
func (f *BlockIssuer) IssuePayload(p payload.Payload, parentsCount ...int) error {

	// TODO:
	// MaxEstimate wait
	// whether to actually wait for booking
	// wait for scheduling
	// number of parents
	// number of strong parents
	// optional references
	//if request.MaxEstimate > 0 && deps.Tangle.RateSetter.Estimate().Milliseconds() > request.MaxEstimate {
	//	return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{
	//		Error: fmt.Sprintf("issuance estimate greater than %d ms", request.MaxEstimate),
	//	})
	//}
	constructedBlock, err := f.Factory.CreateBlock(p, parentsCount...)
	if err != nil {
		return err
	}

	return f.Issue(constructedBlock)
}

// IssuePayloadWithReferences creates a new block with the references submit.
func (f *BlockIssuer) IssuePayloadWithReferences(p payload.Payload, references models.ParentBlockIDs, strongParentsCountOpt ...int) (*models.Block, error) {

}

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBlockFactoryOptions(blockFactoryOptions []options.Option[blockfactory.Factory]) options.Option[BlockIssuer] {
	return func(issuer *BlockIssuer) {
		issuer.optsBlockFactoryOptions = blockFactoryOptions
	}
}
func WithRateSetterOptions(rateSetterOptions []options.Option[ratesetter.RateSetter]) options.Option[BlockIssuer] {
	return func(issuer *BlockIssuer) {
		issuer.optsRateSetterOptions = rateSetterOptions
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
