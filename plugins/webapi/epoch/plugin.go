package epoch

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/hive.go/core/node"

	"net/http"

	"github.com/labstack/echo"
	"go.uber.org/dig"
)

// PluginName is the name of the web API epoch endpoint plugin.
const PluginName = "WebAPIEpochEndpoint"

var (
	// Plugin is the plugin instance of the web API epoch endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Server *echo.Echo
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("epochs", getAllCommittedEpochs)
	deps.Server.GET("ec", getCurrentEC)
	deps.Server.GET("epochs/:ei", getCommittedEpoch)
	deps.Server.GET("epochs/commitment/:commitment", getCommittedEpochByCommitment)
	deps.Server.GET("epochs/:ei/utxos", getUTXOs)
	deps.Server.GET("epochs/:ei/blocks", getBlocks)
	deps.Server.GET("epochs/:ei/transactions", getTransactions)
	deps.Server.GET("epochs/:ei/pending-conflict-count", getPendingConflictsCount)
	deps.Server.GET("epochs/:ei/voters-weight", getVotersWeight)
}

func randRoot() string {
	r := uint64(rand.Int())
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], r)
	hash := blake2b.Sum256(b[:])
	return base58.Encode(hash[:])
}

func randID(withEpochID bool) string {
	r := uint64(rand.Int())
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], r)
	hash := blake2b.Sum256(b[:])
	if withEpochID {
		return fmt.Sprintf("%s:%d", base58.Encode(hash[:]), rand.Intn(1000)+1)
	}
	return base58.Encode(hash[:])
}

func randEpoch() jsonmodels.Epoch {
	start := uint64(rand.Int())
	end := start + 133333333337
	return jsonmodels.Epoch{
		Index:             uint64(rand.Int()),
		Commitment:        randRoot(),
		StartTime:         start,
		EndTime:           end,
		Committed:         true,
		CommitmentRoot:    randRoot(),
		PreviousRoot:      randRoot(),
		NextRoot:          randRoot(),
		TangleRoot:        randRoot(),
		StateMutationRoot: randRoot(),
		StateRoot:         randRoot(),
		ManaRoot:          randRoot(),
		CumulativeStake:   strconv.Itoa(rand.Int()),
	}
}

func getAllCommittedEpochs(c echo.Context) error {
	count := rand.Intn(10) + 1
	epochs := make([]jsonmodels.Epoch, count)
	for i := 0; i < count; i++ {
		epochs[i] = randEpoch()
	}
	return c.JSON(http.StatusOK, epochs)
}

func getCurrentEC(c echo.Context) error {
	return c.JSON(http.StatusOK, randEpoch())
}

func getCommittedEpoch(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	epochInfo := randEpoch()
	epochInfo.Index = uint64(ei)
	return c.JSON(http.StatusOK, epochInfo)
}

func getCommittedEpochByCommitment(c echo.Context) error {
	commitment := c.Param("commitment")
	epochInfo := randEpoch()
	epochInfo.Commitment = commitment
	return c.JSON(http.StatusOK, epochInfo)
}

func randBase58OutputID() string {
	var r [10]byte
	rand.Read(r[:])
	id := utxo.OutputID{
		TransactionID: utxo.NewTransactionID(r[:]),
		Index:         uint16(rand.Intn(5)),
	}
	return id.Base58()
}

func getUTXOs(c echo.Context) error {
	spentCount, createdCount := rand.Intn(10)+1, rand.Intn(10)+1

	spent := make([]string, spentCount)
	for i := 0; i < spentCount; i++ {
		spent[i] = randBase58OutputID()
	}

	created := make([]string, createdCount)
	for i := 0; i < createdCount; i++ {
		created[i] = randBase58OutputID()
	}

	resp := jsonmodels.EpochUTXOsResponse{SpentOutputs: spent, CreatedOutputs: created}

	return c.JSON(http.StatusOK, resp)
}

func getBlocks(c echo.Context) error {
	blocksCount := rand.Intn(20) + 1
	blocks := make([]string, blocksCount)
	for i := 0; i < blocksCount; i++ {
		blocks[i] = randID(true)
	}
	resp := jsonmodels.EpochBlocksResponse{Blocks: blocks}

	return c.JSON(http.StatusOK, resp)
}

func getTransactions(c echo.Context) error {
	txCount := rand.Intn(20) + 1
	txs := make([]string, txCount)
	for i := 0; i < txCount; i++ {
		txs[i] = randID(false)
	}
	resp := jsonmodels.EpochTransactionsResponse{Transactions: txs}

	return c.JSON(http.StatusOK, resp)
}

func getPendingConflictsCount(c echo.Context) error {
	resp := jsonmodels.EpochPendingConflictCountResponse{PendingConflictCount: uint64(rand.Intn(5))}
	return c.JSON(http.StatusOK, resp)
}

func getEI(c echo.Context) (epoch.Index, error) {
	eiText := c.Param("ei")
	eiNumber, err := strconv.Atoi(eiText)
	if err != nil {
		return 0, errors.Wrap(err, "can't parse Index from URL param")
	}
	return epoch.Index(uint64(eiNumber)), nil
}

func getVotersWeight(c echo.Context) error {
	count := rand.Intn(20) + 1
	respMap := make(map[string]*jsonmodels.NodeWeight)
	for i := 0; i < count; i++ {
		ws := make(map[string]float64, 0)
		count2 := rand.Intn(20) + 1
		for j := 0; j < count2; j++ {
			ws[randID(false)] = rand.Float64()
		}
		nodeWeights := &jsonmodels.NodeWeight{Weights: ws}
		respMap[randRoot()] = nodeWeights
	}
	resp := jsonmodels.EpochVotersWeightResponse{VotersWeight: respMap}

	return c.JSON(http.StatusOK, resp)
}
