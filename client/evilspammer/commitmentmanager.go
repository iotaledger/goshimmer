package evilspammer

import (
	"crypto/sha256"
	"math/rand"
	"time"

	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/hive.go/core/slot"

	"github.com/pkg/errors"
)

type CommitmentManagerParams struct {
	CommitmentType    string
	ParentRefsCount   int
	GenesisTime       time.Time
	SlotDuration      time.Duration
	OptionalForkAfter int
}
type CommitmentManager struct {
	Params            *CommitmentManagerParams
	commitmentByIndex map[slot.Index]*commitment.Commitment
	clockSync         *ClockSync

	connector evilwallet.Connector
	forkIndex slot.Index
}

func NewCommitmentManager() *CommitmentManager {
	return &CommitmentManager{
		Params: &CommitmentManagerParams{
			ParentRefsCount: 2,
			GenesisTime:     time.Now(),
			SlotDuration:    5 * time.Second,
		},
	}
}

func (c *CommitmentManager) SetConnector(connector evilwallet.Connector) {
	c.connector = connector
}

func (c *CommitmentManager) SetCommitmentType(commitmentType string) {
	c.Params.CommitmentType = commitmentType
}

func (c *CommitmentManager) SetForkAfter(forkAfter int) {
	c.Params.OptionalForkAfter = forkAfter
}

// SetupForkingPoint sets the forking point for the commitment manager. It uses ForkAfter parameter so need to be called after params are read.
func (c *CommitmentManager) SetupForkingPoint() {
	c.forkIndex = c.clockSync.LatestCommittedSlotClock.Get() + slot.Index(c.Params.OptionalForkAfter)
}

// SetupTimeParams requests through API and sets the genesis time and slot duration for the commitment manager.
func (c *CommitmentManager) SetupTimeParams(clt evilwallet.Client) {
	genesisTime, slotDuration, err := clt.GetTimeProvider()
	if err != nil {
		panic(errors.Wrapf(err, "failed to get time provider for the committment manager setup"))
	}
	c.Params.GenesisTime = genesisTime
	c.Params.SlotDuration = slotDuration
}

// GenerateCommitment generates a commitment based on the commitment type provided in spam details.
func (c *CommitmentManager) GenerateCommitment(clt evilwallet.Client) (*commitment.Commitment, slot.Index, error) {
	switch c.Params.CommitmentType {
	case "latest":
		comm, err := clt.GetLatestCommitment()
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to get latest commitment")
		}
		index, err := clt.GetLatestConfirmedIndex()
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to get latest confirmed index")
		}
		return comm, index, err
	case "random":
		comm, err := clt.GetLatestCommitment()
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to get latest commitment")
		}
		newCommitment := commitment.New(
			comm.Index(),
			comm.PrevID(),
			randomRoot(),
			comm.CumulativeWeight(),
		)
		index, err := clt.GetLatestConfirmedIndex()
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to get latest confirmed index")
		}
		return newCommitment, index, nil
	case "invalid":
		comm := commitment.NewEmptyCommitment()
		newCommitment := commitment.New(
			0,
			comm.PrevID(),
			randomRoot(),
			1000000000,
		)
		return newCommitment, 0, nil
	case "second":
		comm, err := clt.GetLatestCommitment()
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to get latest commitment")
		}
		latestEpoch := comm.ID().SlotIndex
		second := int(latestEpoch) - 1
		secondComm, err := clt.GetCommitment(second)
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to get oldest commitment")
		}
		index, err := clt.GetLatestConfirmedIndex()
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to get latest confirmed index")
		}
		return secondComm, index - 1, nil
	}
	return nil, 0, nil
}

func randomRoot() [32]byte {
	data := make([]byte, 10)
	for i := range data {
		data[i] = byte(rand.Intn(256))
	}
	return sha256.Sum256(data)
}
