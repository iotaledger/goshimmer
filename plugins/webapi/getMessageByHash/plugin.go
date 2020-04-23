package getMessageByHash

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/plugins/webapi"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var PLUGIN = node.NewPlugin("WebAPI getMessageByHash Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-getMessageByHash")
	webapi.Server.POST("getMessageByHash", getMessageByHash)
}

// getMessageByHash returns the array of messages for the
// given message hashes (in the same order as the parameters).
// If a node doesn't have the message for a given message hash in its ledger,
// the value at the index of that message hash is empty.
func getMessageByHash(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	var result []Message
	for _, hash := range request.Hashes {
		log.Info("Received:", hash)

		msgId, err := message.NewId(hash)
		if err != nil {
			log.Info(err)
			continue
		}

		msgObject := messagelayer.Tangle.Message(msgId)
		if !msgObject.Exists() {
			continue
		}

		msg := msgObject.Unwrap()
		msgResp := Message{
			MessageId:       msg.Id().String(),
			TrunkMessageId:  msg.TrunkId().String(),
			BranchMessageId: msg.BranchId().String(),
			IssuerPublicKey: msg.IssuerPublicKey().String(),
			IssuingTime:     msg.IssuingTime().String(),
			SequenceNumber:  msg.SequenceNumber(),
			Payload:         msg.Payload().String(),
			Signature:       msg.Signature().String(),
		}
		result = append(result, msgResp)
		msgObject.Release()
	}

	return c.JSON(http.StatusOK, Response{Messages: result})
}

type Response struct {
	Messages []Message `json:"messages,omitempty"`
	Error    string    `json:"error,omitempty"`
}

type Request struct {
	Hashes []string `json:"hashes"`
}

type Message struct {
	MessageId       string `json:"messageId,omitempty"`
	TrunkMessageId  string `json:"trunkMessageId,omitempty"`
	BranchMessageId string `json:"branchMessageId,omitempty"`
	IssuerPublicKey string `json:"issuerPublicKey,omitempty"`
	IssuingTime     string `json:"issuingTime,omitempty"`
	SequenceNumber  uint64 `json:"sequenceNumber,omitempty"`
	Payload         string `json:"payload,omitempty"`
	Signature       string `json:"signature,omitempty"`
}
