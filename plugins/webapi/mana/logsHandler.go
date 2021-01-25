package mana

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"
)

// getEventLogsHandler handles the request.
func getEventLogsHandler(c echo.Context) error {
	var req GetEventLogsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, GetEventLogsResponse{Error: err.Error()})
	}
	var nodeIDs []identity.ID
	for _, nodeID := range req.NodeIDs {
		_nodeID, err := mana.IDFromStr(nodeID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, GetEventLogsResponse{Error: err.Error()})
		}
		nodeIDs = append(nodeIDs, _nodeID)
	}
	startTime := time.Unix(req.StartTime, 0)
	endTime := time.Unix(req.EndTime, 0)
	epoch := time.Unix(0, 0)
	if endTime == epoch {
		endTime = time.Now()
	}
	if endTime.Before(startTime) {
		return c.JSON(http.StatusBadRequest, GetEventLogsResponse{Error: "time interval mismatch. endTime cannot be before startTime"})
	}
	logs, err := manaPlugin.GetLoggedEvents(nodeIDs, startTime, endTime.Add(1*time.Second))
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetEventLogsResponse{Error: err.Error()})
	}

	res := make(map[string]*EventLogsJSON)
	for ID, l := range logs {
		var pledgesJSON []*mana.PledgedEventJSON
		for _, p := range l.Pledge {
			pledgesJSON = append(pledgesJSON, p.ToJSONSerializable().(*mana.PledgedEventJSON))
		}

		var revokesJSON []*mana.RevokedEventJSON
		for _, r := range l.Revoke {
			revokesJSON = append(revokesJSON, r.ToJSONSerializable().(*mana.RevokedEventJSON))
		}
		eventsJSON := &EventLogsJSON{
			Pledge: pledgesJSON,
			Revoke: revokesJSON,
		}
		res[base58.Encode(ID.Bytes())] = eventsJSON
	}

	return c.JSON(http.StatusOK, GetEventLogsResponse{
		Logs:      res,
		StartTime: startTime.Unix(),
		EndTime:   endTime.Unix(),
	})
}

// EventLogsJSON is a events log in JSON.
type EventLogsJSON struct {
	Pledge []*mana.PledgedEventJSON `json:"pledge"`
	Revoke []*mana.RevokedEventJSON `json:"revoke"`
}

// GetEventLogsRequest is the request.
type GetEventLogsRequest struct {
	NodeIDs   []string `json:"nodeIDs"`
	StartTime int64    `json:"startTime"`
	EndTime   int64    `json:"endTime"`
}

// GetEventLogsResponse is the response.
type GetEventLogsResponse struct {
	Logs      map[string]*EventLogsJSON `json:"logs"`
	Error     string                    `json:"error,omitempty"`
	StartTime int64                     `json:"startTime"`
	EndTime   int64                     `json:"endTime"`
}
