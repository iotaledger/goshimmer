package ledgerstate

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// findBranchByIDHandler returns the array of branches for the
// given branch ids (MUST be encoded in base58), in the same order as the parameters.
// If a node doesn't have the branch for a given ID in its ledger,
// an error is returned.
// If an ID is not base58 encoded, an error is returned
func findBranchByIDHandler(c echo.Context) error {
	var request FindBranchByIDRequest
	if err := c.Bind(&request); err != nil {

		return c.JSON(http.StatusBadRequest, FindBranchByIDResponse{Error: err.Error()})
	}

	var result []Branch
	for _, id := range request.IDs {
		branchID, err := ledgerstate.BranchIDFromBase58(id)
		if err != nil {
			return c.JSON(http.StatusBadRequest, FindBranchByIDResponse{Error: err.Error()})
		}

		if !messagelayer.Tangle().LedgerState.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
			result = append(result, Branch{
				ID:   branchID.Base58(),
				Type: branch.Type().String(),
				Parents: func() []string {
					branchIDs := []string{}
					for id := range branch.Parents() {
						branchIDs = append(branchIDs, id.Base58())
					}
					return branchIDs
				}(),
				ConflictIDs: func() []string {
					if branch.Type() != ledgerstate.ConflictBranchType {
						return []string{}
					}

					conflictIDs := []string{}
					for conflictID := range branch.(*ledgerstate.ConflictBranch).Conflicts() {
						conflictIDs = append(conflictIDs, conflictID.Base58())
					}
					return conflictIDs
				}(),
				Liked:              branch.Liked(),
				MonotonicallyLiked: branch.MonotonicallyLiked(),
				Finalized:          branch.Finalized(),
				InclusionState:     branch.InclusionState().String(),
			})
		}) {
			return c.JSON(http.StatusBadRequest, FindBranchByIDResponse{Error: fmt.Sprintf("Branch with %s does not exist", branchID)})
		}
	}

	return c.JSON(http.StatusOK, FindBranchByIDResponse{Branches: result})
}

// FindBranchByIDResponse is the HTTP response containing the queried branches.
type FindBranchByIDResponse struct {
	Branches []Branch `json:"branches,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// FindBranchByIDRequest holds the branche ids to query.
type FindBranchByIDRequest struct {
	IDs []string `json:"ids"`
}

// Branch contains information about a given branche.
type Branch struct {
	ID                 string   `json:"ID"`
	Type               string   `json:"branchType"`
	Parents            []string `json:"parents"`
	ConflictIDs        []string `json:"conflictIDs,omitempty"`
	Liked              bool     `json:"liked"`
	MonotonicallyLiked bool     `json:"monotonicallyLiked"`
	Finalized          bool     `json:"finalized"`
	InclusionState     string   `json:"inclusionState"`
}
