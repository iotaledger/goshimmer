package jsonmodels

import "github.com/iotaledger/goshimmer/packages/protocol/models"

// region GetReferences Req/Resp /////////////////////////////////////////////////////////////////////////////////////////

// GetReferencesRequest is the HTTP request to get the references for a provided payload.
type GetReferencesRequest struct {
	PayloadBytes []byte `json:"payload_bytes"`
	ParentsCount int    `json:"parent_count"`
}

// GetReferencesResponse is the HTTP response from getting the references for a provided payload.
type GetReferencesResponse struct {
	StrongReferences  []string `json:"references"`
	WeakReferences    []string `json:"weak_references,omitempty"`
	ShallowReferences []string `json:"shallow_references,omitempty"`
}

func (resp *GetReferencesResponse) References() models.ParentBlockIDs {
	refs := make(models.ParentBlockIDs)
	for _, strID := range resp.StrongReferences {
		var blockID models.BlockID
		err := blockID.FromBase58(strID)
		if err != nil {
			return nil
		}
		refs.Add(models.StrongParentType, blockID)
	}
	if resp.WeakReferences != nil {
		for _, strID := range resp.WeakReferences {
			var blockID models.BlockID
			err := blockID.FromBase58(strID)
			if err != nil {
				return nil
			}
			refs.Add(models.WeakParentType, blockID)
		}
	}
	if resp.ShallowReferences != nil {
		for _, strID := range resp.ShallowReferences {
			var blockID models.BlockID
			err := blockID.FromBase58(strID)
			if err != nil {
				return nil
			}
			refs.Add(models.ShallowLikeParentType, blockID)
		}
	}
	return refs
}

// NewGetReferencesResponse creates a new GetReferencesResponse based on provided parents.
func NewGetReferencesResponse(references models.ParentBlockIDs) (resp *GetReferencesResponse) {
	for parentType, blockIDs := range references {
		switch parentType {
		case models.StrongParentType:
			strong := make([]string, 0)
			for blockID := range blockIDs {
				strong = append(strong, blockID.Base58())
			}
			resp.StrongReferences = strong
		case models.WeakParentType:
			weak := make([]string, 0)
			for blockID := range blockIDs {
				weak = append(weak, blockID.Base58())
			}
			resp.WeakReferences = weak
		case models.ShallowLikeParentType:
			shallow := make([]string, 0)
			for blockID := range blockIDs {
				shallow = append(shallow, blockID.Base58())
			}
			resp.ShallowReferences = shallow
		}
	}
	return resp
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
