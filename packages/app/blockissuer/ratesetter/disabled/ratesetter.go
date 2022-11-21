package disabled

import (
	"github.com/iotaledger/goshimmer/packages/app/blockissuer/ratesetter/utils"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"time"
)

// region DisabledRateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

type RateSetter struct {
	events       *utils.Events
	issuingQueue *utils.IssuerQueue
}

func New() *RateSetter {
	return &RateSetter{
		events: utils.NewEvents(),
	}
}

func (r *RateSetter) Shutdown() {
	// shutdown?
}

func (r *RateSetter) Rate() float64 {
	return 0
}
func (r *RateSetter) Estimate() time.Duration {
	return time.Duration(0)
}
func (r *RateSetter) Size() int {
	return r.issuingQueue.Size()
}
func (r *RateSetter) Events() *utils.Events {
	return r.events
}

func (r *RateSetter) SubmitBlock(block *models.Block) error {
	r.events.BlockIssued.Trigger(block)
	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
