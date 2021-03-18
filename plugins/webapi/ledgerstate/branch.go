package ledgerstate

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/labstack/echo"
)

// region API endpoints ////////////////////////////////////////////////////////////////////////////////////////////////

// GetBranchEndPoint is the handler for the /ledgerstate/branch/:branchID endpoint.
func GetBranchEndPoint(c echo.Context) (err error) {
	branchID, err := branchIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, webapi.NewErrorResponse(err))
	}

	if messagelayer.Tangle().LedgerState.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		err = c.JSON(http.StatusOK, NewBranch(branch))
	}) {
		return
	}

	return c.JSON(http.StatusBadRequest, webapi.NewErrorResponse(fmt.Errorf("failed to load Branch with %s", branchID)))
}

// GetBranchChildrenEndPoint is the handler for the /ledgerstate/branch/:branchID/childBranches endpoint.
func GetBranchChildrenEndPoint(c echo.Context) (err error) {
	branchID, err := branchIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, webapi.NewErrorResponse(err))
	}

	cachedChildBranches := messagelayer.Tangle().LedgerState.ChildBranches(branchID)
	defer cachedChildBranches.Release()

	return c.JSON(http.StatusOK, NewBranchChildren(branchID, cachedChildBranches.Unwrap()))
}

func GetBranchConflictsEndPoint(c echo.Context) (err error) {
	branchID, err := branchIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}

	if messagelayer.Tangle().LedgerState.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		if branch.Type() != ledgerstate.ConflictBranchType {
			err = c.JSON(http.StatusBadRequest, NewErrorResponse(fmt.Errorf("the Branch with %s is not a ConflictBranch", branchID)))
			return
		}

		err = c.JSON(http.StatusOK, NewBranchConflicts(branch.(*ledgerstate.ConflictBranch)))
	}) {
		return
	}

	return c.JSON(http.StatusBadRequest, NewErrorResponse(fmt.Errorf("failed to load Branch with %s", branchID)))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Branch ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Branch represents the JSON model of a ledgerstate.Branch.
type Branch struct {
	ID                 string   `json:"id"`
	Type               string   `json:"type"`
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
	BranchID      string        `json:"branchID"`
	ChildBranches []ChildBranch `json:"childBranches"`
}

// NewBranchChildren returns BranchChildren from the given collection of ledgerstate.ChildBranch objects.
func NewBranchChildren(branchID ledgerstate.BranchID, childBranches []*ledgerstate.ChildBranch) BranchChildren {
	return BranchChildren{
		BranchID: branchID.Base58(),
		ChildBranches: func() (mappedChildBranches []ChildBranch) {
			mappedChildBranches = make([]ChildBranch, 0)
			for _, childBranch := range childBranches {
				mappedChildBranches = append(mappedChildBranches, NewChildBranch(childBranch))
			}

			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ChildBranch //////////////////////////////////////////////////////////////////////////////////////////////////

// ChildBranch represents the JSON model of a ledgerstate.ChildBranch.
type ChildBranch struct {
	BranchID   string `json:"branchID"`
	BranchType string `json:"type"`
}

// NewChildBranch returns a ChildBranch from the given ledgerstate.ChildBranch.
func NewChildBranch(childBranch *ledgerstate.ChildBranch) ChildBranch {
	return ChildBranch{
		BranchID:   childBranch.ChildBranchID().Base58(),
		BranchType: childBranch.ChildBranchType().String(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchConflicts //////////////////////////////////////////////////////////////////////////////////////////////

// BranchConflicts represents the JSON model of a collection of Conflicts that a ledgerstate.ConflictBranch is part of.
type BranchConflicts struct {
	BranchID  string     `json:"branchID"`
	Conflicts []Conflict `json:"conflicts"`
}

// NewBranchConflicts returns the BranchConflicts that a ledgerstate.ConflictBranch is part of.
func NewBranchConflicts(conflictBranch *ledgerstate.ConflictBranch) BranchConflicts {
	return BranchConflicts{
		BranchID: conflictBranch.ID().Base58(),
		Conflicts: func() (mappedConflicts []Conflict) {
			mappedConflicts = make([]Conflict, 0)
			for conflictID := range conflictBranch.Conflicts() {
				mappedConflicts = append(mappedConflicts, NewConflict(conflictID))
			}

			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

type OutputID struct {
	ID            string `json:"id"`
	TransactionID string `json:"transactionID"`
	OutputIndex   uint16 `json:"outputIndex"`
}

// NewOutputID returns a OutputID from the given ledgerstate.OutputID.
func NewOutputID(outputID ledgerstate.OutputID) OutputID {
	return OutputID{
		ID:            outputID.Base58(),
		TransactionID: outputID.TransactionID().Base58(),
		OutputIndex:   outputID.OutputIndex(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Conflict /////////////////////////////////////////////////////////////////////////////////////////////////////

// Conflict represents the JSON model of a ledgerstate.Conflict.
type Conflict struct {
	OutputID  OutputID `json:"outputID"`
	BranchIDs []string `json:"branchIDs"`
}

// NewConflict returns a Conflict from the given ledgerstate.ConflictID.
func NewConflict(conflictID ledgerstate.ConflictID) Conflict {
	return Conflict{
		OutputID: NewOutputID(conflictID.OutputID()),
		BranchIDs: func() (mappedBranchIDs []string) {
			mappedBranchIDs = make([]string, 0)
			messagelayer.Tangle().LedgerState.BranchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
				mappedBranchIDs = append(mappedBranchIDs, conflictMember.BranchID().Base58())
			})

			return
		}(),
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
