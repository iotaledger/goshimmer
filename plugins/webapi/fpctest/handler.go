package fpctest

import (
	"crypto/rand"
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/fpctest/packages/payload"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/labstack/echo"
)

// Handler issue a new FPCTest message.
func Handler(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	var ids []string
	for i := 0; i < request.Amount; i++ {
		nonce := make([]byte, payload.NonceSize)
		_, err := rand.Read(nonce)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, Response{Error: err.Error()})
		}
		p := payload.New(request.Like, nonce)

		msg, err := issuer.IssuePayload(p)
		if err != nil {
			return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
		}
		ids = append(ids, msg.Id().String())
	}
	return c.JSON(http.StatusOK, Response{IDs: ids})
}

// Response is the HTTP response from broadcasting a FPCTest message.
type Response struct {
	IDs   []string `json:"ids,omitempty"`
	Error string   `json:"error,omitempty"`
}

// Request is a request containing a collective beacon response.
type Request struct {
	Like   uint32 `json:"like"`
	Amount int    `json:"amount"`
}
