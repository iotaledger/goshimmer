package ledgerstate

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// GetBranchResponse is the response object of the GetBranch function.
type GetBranchResponse struct {
	Branch Branch `json:"branch,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Branch represents the JSON model of a ledgerstate.Branch.
type Branch struct {
	ID                 string   `json:"id"`
	Type               string   `json:"branchType"`
	Parents            []string `json:"parents"`
	ConflictIDs        []string `json:"conflictIDs,omitempty"`
	Liked              bool     `json:"liked"`
	MonotonicallyLiked bool     `json:"monotonicallyLiked"`
	Finalized          bool     `json:"finalized"`
	InclusionState     string   `json:"inclusionState"`
}

// BranchFromModel returns the JSON model of a ledgerstate.Branch.
func BranchFromModel(branch ledgerstate.Branch) Branch {
	return Branch{
		ID:   branch.ID().Base58(),
		Type: branch.Type().String(),
		Parents: func() []string {
			parents := make([]string, 0)
			for id := range branch.Parents() {
				parents = append(parents, id.Base58())
			}

			return parents
		}(),
		ConflictIDs: func() []string {
			if branch.Type() != ledgerstate.ConflictBranchType {
				return make([]string, 0)
			}

			conflictIDs := make([]string, 0)
			for conflictID := range branch.(*ledgerstate.ConflictBranch).Conflicts() {
				conflictIDs = append(conflictIDs, conflictID.Base58())
			}

			return conflictIDs
		}(),
		Liked:              branch.Liked(),
		MonotonicallyLiked: branch.MonotonicallyLiked(),
		Finalized:          branch.Finalized(),
		InclusionState:     branch.InclusionState().String(),
	}
}

// getBranch is the handler for the /ledgerstate/branch/:branchID API endpoint. It expects the branchID to be passed in
// as a base58 encoded string.
func getBranch(c echo.Context) (err error) {
	branchID, err := branchIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetBranchResponse{Error: err.Error()})
	}

	if messagelayer.Tangle().LedgerState.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		_ = c.JSON(http.StatusOK, BranchFromModel(branch))
	}) {
		return
	}

	return c.JSON(http.StatusBadRequest, GetBranchResponse{Error: fmt.Sprintf("Branch with %s does not exist", branchID)})
}

// branchIDFromContext returns the BranchID from a request context.
func branchIDFromContext(c echo.Context) (branchID ledgerstate.BranchID, err error) {
	switch branchIDString := c.Param("branchID"); branchIDString {
	case "MasterBranchID":
		branchID = ledgerstate.MasterBranchID
	case "LazyBookedConflictsBranchID":
		branchID = ledgerstate.LazyBookedConflictsBranchID
	case "InvalidBranchID":
		branchID = ledgerstate.InvalidBranchID
	default:
		branchID, err = ledgerstate.BranchIDFromBase58(branchIDString)
	}

	return
}
