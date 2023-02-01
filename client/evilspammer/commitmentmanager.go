package evilspammer

import (
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/pkg/errors"
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

// GenerateCommitment generates a commitment based on the commitment type provided in spam details.
func (c *CommitmentManager) GenerateCommitment() (*commitment.Commitment, epoch.Index, error) {
	clt := c.connector.GetClient()
	switch c.CommitmentType {
	case "latest":
		resp, err := clt.GetLatestCommitment()
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to get latest commitment")
		}
		var commitment *commitment.Commitment
		_, err = commitment.FromBytes(resp.Bytes)
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to parse commitment bytes")
		}
		return commitment, epoch.Index(resp.LatestConfirmedIndex), err
	case "random":
	case "oldest":

	}
	return nil, 0, nil
}
