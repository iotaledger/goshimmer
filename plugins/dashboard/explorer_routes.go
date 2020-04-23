package dashboard

import (
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
	"github.com/pkg/errors"
)

// ExplorerMessage defines the struct of the ExplorerMessage.
type ExplorerMessage struct {
	// ID is the message ID.
	ID string `json:"id"`
	// Timestamp is the timestamp of the message.
	Timestamp uint `json:"timestamp"`
	// TrunkMessageId is the Trunk ID of the message.
	TrunkMessageID string `json:"trunk_message_id"`
	// BranchMessageId is the Branch ID of the message.
	BranchMessageID string `json:"branch_message_id"`
	// Solid defines the solid status of the message.
	Solid bool `json:"solid"`
}

func createExplorerMessage(msg *message.Message) (*ExplorerMessage, error) {
	messageID := msg.Id()
	messageMetadata := messagelayer.Tangle.MessageMetadata(messageID)
	t := &ExplorerMessage{
		ID:              messageID.String(),
		Timestamp:       0,
		TrunkMessageID:  msg.TrunkId().String(),
		BranchMessageID: msg.BranchId().String(),
		Solid:           messageMetadata.Unwrap().IsSolid(),
	}

	return t, nil
}

// ExplorerAddress defines the struct of the ExplorerAddress.
type ExplorerAddress struct {
	// Messagess hold the list of *ExplorerMessage.
	Messages []*ExplorerMessage `json:"message"`
}

// SearchResult defines the struct of the SearchResult.
type SearchResult struct {
	// Message is the *ExplorerMessage.
	Message *ExplorerMessage `json:"message"`
	// Address is the *ExplorerAddress.
	Address *ExplorerAddress `json:"address"`
}

func setupExplorerRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/message/:id", func(c echo.Context) (err error) {
		messageID, err := message.NewId(c.Param("id"))
		if err != nil {
			return
		}

		t, err := findMessage(messageID)
		if err != nil {
			return
		}

		return c.JSON(http.StatusOK, t)
	})

	routeGroup.GET("/address/:id", func(c echo.Context) error {
		addr, err := findAddress(c.Param("id"))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, addr)
	})

	routeGroup.GET("/search/:search", func(c echo.Context) error {
		search := c.Param("search")
		result := &SearchResult{}

		if len(search) < 81 {
			return errors.Wrapf(ErrInvalidParameter, "search id invalid: %s", search)
		}

		wg := sync.WaitGroup{}
		wg.Add(2)
		go func() {
			defer wg.Done()

			messageID, err := message.NewId(search)
			if err != nil {
				return
			}

			msg, err := findMessage(messageID)
			if err == nil {
				result.Message = msg
			}
		}()

		go func() {
			defer wg.Done()
			addr, err := findAddress(search)
			if err == nil {
				result.Address = addr
			}
		}()
		wg.Wait()

		return c.JSON(http.StatusOK, result)
	})
}

func findMessage(messageID message.Id) (explorerMsg *ExplorerMessage, err error) {
	if !messagelayer.Tangle.Message(messageID).Consume(func(msg *message.Message) {
		explorerMsg, err = createExplorerMessage(msg)
	}) {
		err = errors.Wrapf(ErrNotFound, "message: %s", messageID.String())
	}

	return
}

func findAddress(address string) (*ExplorerAddress, error) {
	return nil, errors.Wrapf(ErrNotFound, "address %s not found", address)

	// TODO: ADD ADDRESS LOOKUPS ONCE THE VALUE TRANSFER ONTOLOGY IS MERGED
}
