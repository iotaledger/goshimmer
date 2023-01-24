package evilspammer

import (
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type CommitmentManager struct {
	commitmentType string
	connector      evilwallet.Connector
}

func NewCommitmentManager() *CommitmentManager {
	return &CommitmentManager{}
}

func (c *CommitmentManager) SetConnector(connector evilwallet.Connector) {
	c.connector = connector
}

func (c *CommitmentManager) SetCommitmentType(commitmentType string) {
	c.commitmentType = commitmentType
}

func (c *CommitmentManager) GenerateCommitment() (*commitment.Commitment, epoch.Index, error) {
	switch c.commitmentType {
	case "latest":
	case "random":

	}
	return nil, 0, nil
}
