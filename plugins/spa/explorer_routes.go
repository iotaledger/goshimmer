package spa

import (
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"

	"github.com/labstack/echo"
	"github.com/pkg/errors"
)

type ExplorerMessage struct {
	Id              string `json:"id"`
	Timestamp       uint   `json:"timestamp"`
	TrunkMessageId  string `json:"trunk_message_id"`
	BranchMessageId string `json:"branch_message_id"`
	Solid           bool   `json:"solid"`
}

func createExplorerMessage(msg *message.Message) (*ExplorerMessage, error) {
	messageId := msg.Id()
	messageMetadata := messagelayer.Tangle.MessageMetadata(messageId)
	t := &ExplorerMessage{
		Id:              messageId.String(),
		Timestamp:       0,
		TrunkMessageId:  msg.TrunkId().String(),
		BranchMessageId: msg.BranchId().String(),
		Solid:           messageMetadata.Unwrap().IsSolid(),
	}

	return t, nil
}

type ExplorerAddress struct {
	Messages []*ExplorerMessage `json:"message"`
}

type SearchResult struct {
	Message *ExplorerMessage `json:"message"`
	Address *ExplorerAddress `json:"address"`
}

func setupExplorerRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/message/:id", func(c echo.Context) (err error) {
		messageId, err := message.NewId(c.Param("id"))
		if err != nil {
			return
		}

		t, err := findMessage(messageId)
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

			messageId, err := message.NewId(search)
			if err != nil {
				return
			}

			msg, err := findMessage(messageId)
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

func findMessage(messageId message.Id) (explorerMsg *ExplorerMessage, err error) {
	if !messagelayer.Tangle.Message(messageId).Consume(func(msg *message.Message) {
		explorerMsg, err = createExplorerMessage(msg)
	}) {
		err = errors.Wrapf(ErrNotFound, "message: %s", messageId.String())
	}

	return
}

func findAddress(address string) (*ExplorerAddress, error) {
	return nil, errors.Wrapf(ErrNotFound, "address %s not found", address)

	// TODO: ADD ADDRESS LOOKUPS ONCE THE VALUE TRANSFER ONTOLOGY IS MERGED
}
