package fpctest

import (
	"crypto/rand"
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/fpctest/packages/payload"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

// Handler issue a new FPCTest message.
func Handler(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	nonce := make([]byte, payload.NonceSize)
	rand.Read(nonce)
	p := payload.New(request.Like, nonce)

	msg, err := issuer.IssuePayload(p)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, Response{ID: msg.Id().String()})
}

// Response is the HTTP response from broadcasting a FPCTest message.
type Response struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// Request is a request containing a collective beacon response.
type Request struct {
	Like uint32 `json:"like"`
}
