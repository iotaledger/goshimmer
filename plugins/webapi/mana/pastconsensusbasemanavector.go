package mana

//import (
//	"net/http"
//	"time"
//
//	"github.com/labstack/echo"
//
//	manaPlugin "github.com/iotaledger/goshimmer/plugins/messagelayer"
//	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
//)
//
//// getPastConsensusManaVectorHandler handles the request.
//func getPastConsensusManaVectorHandler(c echo.Context) error {
//	var req jsonmodels.PastConsensusManaVectorRequest
//	if err := c.Bind(&req); err != nil {
//		return c.JSON(http.StatusBadRequest, jsonmodels.PendingResponse{Error: err.Error()})
//	}
//	timestamp := time.Unix(req.Timestamp, 0)
//	consensus, _, err := manaPlugin.GetPastConsensusManaVector(timestamp.Add(1 * time.Second))
//	if err != nil {
//		return c.JSON(http.StatusBadRequest, jsonmodels.PastConsensusManaVectorResponse{Error: err.Error()})
//	}
//	manaMap, _, err := consensus.GetManaMap(timestamp)
//	if err != nil {
//		return c.JSON(http.StatusBadRequest, jsonmodels.PastConsensusManaVectorResponse{Error: err.Error()})
//	}
//
//	return c.JSON(http.StatusOK, jsonmodels.PastConsensusManaVectorResponse{
//		Consensus: manaMap.ToNodeStrList(),
//		TimeStamp: timestamp.Unix(),
//	})
//}
