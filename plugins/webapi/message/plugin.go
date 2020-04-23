package message

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/labstack/echo"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the web API message endpoint plugin.
const PluginName = "WebAPI message Endpoint"

var (
	// Plugin is the plugin instance of the web API message endpoint plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	log    *logger.Logger
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
	webapi.Server.POST("message/findById", findMessageById)
}

// findMessageById returns the array of messages for the
// given message ids (MUST be encoded in base58), in the same order as the parameters.
// If a node doesn't have the message for a given ID in its ledger,
// the value at the index of that message ID is empty.
func findMessageById(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	var result []Message
	for _, id := range request.Ids {
		log.Info("Received:", id)

		msgId, err := message.NewId(id)
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
			Id:              msg.Id().String(),
			TrunkId:         msg.TrunkId().String(),
			BranchId:        msg.BranchId().String(),
			IssuerPublicKey: msg.IssuerPublicKey().String(),
			IssuingTime:     msg.IssuingTime().String(),
			SequenceNumber:  msg.SequenceNumber(),
			Payload:         msg.Payload().Bytes(),
			Signature:       msg.Signature().String(),
		}
		result = append(result, msgResp)
		msgObject.Release()
	}

	return c.JSON(http.StatusOK, Response{Messages: result})
}

// Response is the HTTP response containing the queried messages.
type Response struct {
	Messages []Message `json:"messages,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// Request holds the message ids to query.
type Request struct {
	Ids []string `json:"ids"`
}

// Message contains information about a given message.
type Message struct {
	Id              string `json:"Id,omitempty"`
	TrunkId         string `json:"trunkId,omitempty"`
	BranchId        string `json:"branchId,omitempty"`
	IssuerPublicKey string `json:"issuerPublicKey,omitempty"`
	IssuingTime     string `json:"issuingTime,omitempty"`
	SequenceNumber  uint64 `json:"sequenceNumber,omitempty"`
	Payload         []byte `json:"payload,omitempty"`
	Signature       string `json:"signature,omitempty"`
}
