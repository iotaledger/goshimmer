package mana

//// getPastConsensusVectorMetadataHandler handles the request.
//func getPastConsensusVectorMetadataHandler(c echo.Context) error {
//	metadata := manaPlugin.GetPastConsensusManaVectorMetadata()
//	if metadata == nil {
//		return c.JSON(http.StatusOK, jsonmodels.PastConsensusVectorMetadataResponse{
//			Error: "Past consensus mana vector metadata not found",
//		})
//	}
//	return c.JSON(http.StatusOK, jsonmodels.PastConsensusVectorMetadataResponse{
//		Metadata: metadata,
//	})
//}
