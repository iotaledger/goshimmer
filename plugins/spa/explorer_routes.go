package spa

import (
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/plugins/tangle"

	"github.com/labstack/echo"
	"github.com/pkg/errors"
)

type ExplorerTx struct {
	Hash                     string `json:"hash"`
	SignatureMessageFragment string `json:"signature_message_fragment"`
	Address                  string `json:"address"`
	Value                    int64  `json:"value"`
	Timestamp                uint   `json:"timestamp"`
	Trunk                    string `json:"trunk"`
	Branch                   string `json:"branch"`
	Solid                    bool   `json:"solid"`
	MWM                      int    `json:"mwm"`
}

func createExplorerTx(tx *message.Transaction) (*ExplorerTx, error) {
	transactionId := tx.GetId()

	txMetadata := tangle.Instance.GetTransactionMetadata(transactionId)

	t := &ExplorerTx{
		Hash:                     transactionId.String(),
		SignatureMessageFragment: "",
		Address:                  "",
		Timestamp:                0,
		Value:                    0,
		Trunk:                    tx.GetTrunkTransactionId().String(),
		Branch:                   tx.GetBranchTransactionId().String(),
		Solid:                    txMetadata.Unwrap().IsSolid(),
	}

	// TODO: COMPUTE MWM
	t.MWM = 0

	return t, nil
}

type ExplorerAdress struct {
	Txs []*ExplorerTx `json:"txs"`
}

type SearchResult struct {
	Tx        *ExplorerTx     `json:"tx"`
	Address   *ExplorerAdress `json:"address"`
	Milestone *ExplorerTx     `json:"milestone"`
}

func setupExplorerRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/tx/:hash", func(c echo.Context) (err error) {
		transactionId, err := message.NewId(c.Param("hash"))
		if err != nil {
			return
		}

		t, err := findTransaction(transactionId)
		if err != nil {
			return
		}

		return c.JSON(http.StatusOK, t)
	})

	routeGroup.GET("/addr/:hash", func(c echo.Context) error {
		addr, err := findAddress(c.Param("hash"))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, addr)
	})

	routeGroup.GET("/search/:search", func(c echo.Context) error {
		search := c.Param("search")
		result := &SearchResult{}

		if len(search) < 81 {
			return errors.Wrapf(ErrInvalidParameter, "search hash invalid: %s", search)
		}

		wg := sync.WaitGroup{}
		wg.Add(2)
		go func() {
			defer wg.Done()

			transactionId, err := message.NewId(search)
			if err != nil {
				return
			}

			tx, err := findTransaction(transactionId)
			if err == nil {
				result.Tx = tx
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

func findTransaction(transactionId message.Id) (explorerTx *ExplorerTx, err error) {
	if !tangle.Instance.GetTransaction(transactionId).Consume(func(transaction *message.Transaction) {
		explorerTx, err = createExplorerTx(transaction)
	}) {
		err = errors.Wrapf(ErrNotFound, "tx hash: %s", transactionId.String())
	}

	return
}

func findAddress(address string) (*ExplorerAdress, error) {
	return nil, errors.Wrapf(ErrNotFound, "address %s not found", address)

	// TODO: ADD ADDRESS LOOKUPS ONCE THE VALUE TRANSFER ONTOLOGY IS MERGED

	/*
		if len(hash) > 81 {
			hash = hash[:81]
		}
		if !guards.IsTrytesOfExactLength(hash, consts.HashTrytesSize) {
			return nil, errors.Wrapf(ErrInvalidParameter, "hash invalid: %s", hash)
		}

		txHashes, err := tangle_old.ReadTransactionHashesForAddressFromDatabase(hash)
		if err != nil {
			return nil, ErrInternalError
		}

		if len(txHashes) == 0 {
			return nil, errors.Wrapf(ErrNotFound, "address %s not found", hash)
		}

		txs := make([]*ExplorerTx, 0, len(txHashes))
		for i := 0; i < len(txHashes); i++ {
			txHash := txHashes[i]

			tx, err := tangle_old.GetTransaction(hash)
			if err != nil {
				continue
			}
			if tx == nil {
				continue
			}
			expTx, err := createExplorerTx(txHash, tx)
			if err != nil {
				return nil, err
			}
			txs = append(txs, expTx)
		}

		return &ExplorerAdress{Txs: txs}, nil
	*/
}
