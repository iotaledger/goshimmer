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
	if metadata == nil {
		return c.JSON(http.StatusOK, PastConsensusVectorMetadataResponse{
			Error: "Past consensus mana vector metadata not found",
		})
	}
	return c.JSON(http.StatusOK, PastConsensusVectorMetadataResponse{
		Metadata: metadata,
	})
}

// PastConsensusVectorMetadataResponse is the response.
type PastConsensusVectorMetadataResponse struct {
	Metadata *mana.ConsensusBasePastManaVectorMetadata `json:"metadata,omitempty"`
	Error    string                                    `json:"error,omitempty"`
}
