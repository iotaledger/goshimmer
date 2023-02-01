package evilspammer

import (
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type CommitmentManager struct {
	CommitmentType  string
	ParentRefsCount int
	connector       evilwallet.Connector
}

func NewCommitmentManager() *CommitmentManager {
	return &CommitmentManager{
		ParentRefsCount: 2,
	}
}

func (c *CommitmentManager) SetConnector(connector evilwallet.Connector) {
	c.connector = connector
}

func (c *CommitmentManager) SetCommitmentType(commitmentType string) {
	c.CommitmentType = commitmentType
}

func (c *CommitmentManager) GenerateCommitment() (*commitment.Commitment, epoch.Index, error) {
	switch c.CommitmentType {
	case "latest":
	case "random":

	}
	return nil, 0, nil
}
