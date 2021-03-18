package ledgerstate

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// region API endpoints ////////////////////////////////////////////////////////////////////////////////////////////////

// GetBranchEndPoint is the handler for the /ledgerstate/branch/:branchID endpoint.
func GetBranchEndPoint(c echo.Context) (err error) {
	branchID, err := branchIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}

	if messagelayer.Tangle().LedgerState.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		err = c.JSON(http.StatusOK, NewBranch(branch))
	}) {
		return
	}

	return c.JSON(http.StatusBadRequest, NewErrorResponse(fmt.Errorf("failed to load Branch with %s", branchID)))
}

// GetBranchChildrenEndPoint is the handler for the /ledgerstate/branch/:branchID/childBranches endpoint.
func GetBranchChildrenEndPoint(c echo.Context) (err error) {
	branchID, err := branchIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}

	cachedChildBranches := messagelayer.Tangle().LedgerState.ChildBranches(branchID)
	defer cachedChildBranches.Release()

	return c.JSON(http.StatusOK, NewBranchChildren(cachedChildBranches.Unwrap()))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Branch ///////////////////////////////////////////////////////////////////////////////////////////////////////

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

// NewBranch returns a Branch from the given ledgerstate.Branch.
func NewBranch(branch ledgerstate.Branch) Branch {
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchChildren ///////////////////////////////////////////////////////////////////////////////////////////////

// BranchChildren represents the JSON model of a collection of ChildBranch objects.
type BranchChildren struct {
	Children []ChildBranch `json:"childBranches"`
}

// NewBranchChildren returns BranchChildren from the given collection of ledgerstate.ChildBranch objects.
func NewBranchChildren(childBranches []*ledgerstate.ChildBranch) BranchChildren {
	return BranchChildren{
		Children: func() (children []ChildBranch) {
			children = make([]ChildBranch, 0)
			for _, childBranch := range childBranches {
				children = append(children, NewChildBranch(childBranch))
			}

			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ChildBranch //////////////////////////////////////////////////////////////////////////////////////////////////

// ChildBranch represents the JSON model of a ledgerstate.ChildBranch.
type ChildBranch struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// NewChildBranch returns a ChildBranch from the given ledgerstate.ChildBranch.
func NewChildBranch(childBranch *ledgerstate.ChildBranch) ChildBranch {
	return ChildBranch{
		ID:   childBranch.ChildBranchID().Base58(),
		Type: childBranch.ChildBranchType().String(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region branchIDFromContext //////////////////////////////////////////////////////////////////////////////////////////

// branchIDFromContext determines the BranchID from the branchID parameter in an echo.Context. It expects it to either
// be a base58 encoded string or one of the builtin aliases (MasterBranchID, LazyBookedConflictsBranchID or
// InvalidBranchID)
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
