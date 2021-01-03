package mana

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// getPastConsensusVectorMetadataHandler handles the request.
func getPastConsensusVectorMetadataHandler(c echo.Context) error {
	metadata := manaPlugin.GetPastConsensusManaVectorMetadata()

	return c.JSON(http.StatusOK, PastConsensusVectorMetadataResponse{
		Metadata: metadata,
	})
}

// PastConsensusVectorMetadataResponse is the response.
type PastConsensusVectorMetadataResponse struct {
	Metadata mana.ConsensusBasePastManaVectorMetadata `json:"metadata"`
	Error    string                                   `json:"error,omitempty"`
}
