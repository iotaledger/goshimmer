package message

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

// MissingHandler process missing requests.
func MissingHandler(c echo.Context) error {
	res := &jsonmodels.MissingResponse{}
	missingIDs := deps.Tangle.Storage.MissingMessages()
	for _, msg := range missingIDs {
		res.IDs = append(res.IDs, msg.Base58())
	}
	res.Count = len(missingIDs)
	return c.JSON(http.StatusOK, res)
}

// MissingAvailableHandler is the handler for requests that check if a Message is available at a nodes neighbors.
func MissingAvailableHandler(c echo.Context) error {
	maxNum, err := strconv.Atoi(c.QueryParam("maxNum"))
	if err != nil {
		maxNum = 0
	}
	result := &jsonmodels.MissingAvailableResponse{}
	missingIDs := deps.Tangle.Storage.MissingMessages()
	if maxNum > 0 && len(missingIDs) > maxNum {
		missingIDs = missingIDs[:maxNum]
	}
	peersEndpoints := make(map[string]*client.GoShimmerAPI)
	if deps.GossipMgr != nil {
		for _, neighbor := range deps.GossipMgr.AllNeighbors() {
			timedClient := http.Client{
				Timeout: 5 * time.Second,
			}
			peersEndpoints[neighbor.ID().String()] = client.NewGoShimmerAPI("http://"+neighbor.IP().String()+":8080", client.WithHTTPClient(timedClient))
		}
	}

	msgAvailability := make(map[string][]string)
	var msgAvailabilityLock sync.Mutex
	var wg sync.WaitGroup
	for _, missingID := range missingIDs {
		missingIDBase58 := missingID.Base58()
		msgAvailability[missingIDBase58] = make([]string, 0)
		for peerID, c := range peersEndpoints {
			wg.Add(1)
			go func(peerId string, c *client.GoShimmerAPI) {
				fmt.Println("Querying", peerId, "about", missingIDBase58)
				_, err := c.GetMessageMetadata(missingIDBase58)
				if err == nil {
					msgAvailabilityLock.Lock()
					msgAvailability[missingIDBase58] = append(msgAvailability[missingIDBase58], peerId)
					msgAvailabilityLock.Unlock()
				}
				wg.Done()
			}(peerID, c)
		}
	}
	wg.Wait()
	result.Availability = msgAvailability
	result.Count = len(missingIDs)
	return c.JSON(http.StatusOK, result)
}
