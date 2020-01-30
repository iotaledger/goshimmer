package spa

import (
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/packages/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/labstack/echo"
	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/guards"
	. "github.com/iotaledger/iota.go/trinary"
)

type ExplorerTx struct {
	Hash                     Hash   `json:"hash"`
	SignatureMessageFragment Trytes `json:"signature_message_fragment"`
	Address                  Hash   `json:"address"`
	Value                    int64  `json:"value"`
	Timestamp                uint   `json:"timestamp"`
	Trunk                    Hash   `json:"trunk"`
	Branch                   Hash   `json:"branch"`
	Solid                    bool   `json:"solid"`
	MWM                      int    `json:"mwm"`
}

func createExplorerTx(hash Hash, tx *value_transaction.ValueTransaction) (*ExplorerTx, error) {

	txMetadata, err := tangle.GetTransactionMetadata(hash, transactionmetadata.New)
	if err != nil {
		return nil, err
	}

	t := &ExplorerTx{
		Hash:                     tx.GetHash(),
		SignatureMessageFragment: tx.GetSignatureMessageFragment(),
		Address:                  tx.GetAddress(),
		Timestamp:                tx.GetTimestamp(),
		Value:                    tx.GetValue(),
		Trunk:                    tx.GetTrunkTransactionHash(),
		Branch:                   tx.GetBranchTransactionHash(),
		Solid:                    txMetadata.GetSolid(),
	}

	// compute mwm
	trits := MustTrytesToTrits(hash)
	var mwm int
	for i := len(trits) - 1; i >= 0; i-- {
		if trits[i] == 0 {
			mwm++
			continue
		}
		break
	}
	t.MWM = mwm
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

	routeGroup.GET("/tx/:hash", func(c echo.Context) error {
		t, err := findTransaction(c.Param("hash"))
		if err != nil {
			return err
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

		// auto. remove checksum
		search = search[:81]

		wg := sync.WaitGroup{}
		wg.Add(2)
		go func() {
			defer wg.Done()
			tx, err := findTransaction(search)
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

func findTransaction(hash Hash) (*ExplorerTx, error) {
	if !guards.IsTrytesOfExactLength(hash, consts.HashTrytesSize) {
		return nil, errors.Wrapf(ErrInvalidParameter, "hash invalid: %s", hash)
	}

	tx, err := tangle.GetTransaction(hash)
	if err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, errors.Wrapf(ErrNotFound, "tx hash: %s", hash)
	}

	t, err := createExplorerTx(hash, tx)
	return t, err
}

func findAddress(hash Hash) (*ExplorerAdress, error) {
	if len(hash) > 81 {
		hash = hash[:81]
	}
	if !guards.IsTrytesOfExactLength(hash, consts.HashTrytesSize) {
		return nil, errors.Wrapf(ErrInvalidParameter, "hash invalid: %s", hash)
	}

	txHashes, err := tangle.ReadTransactionHashesForAddressFromDatabase(hash)
	if err != nil {
		return nil, ErrInternalError
	}

	if len(txHashes) == 0 {
		return nil, errors.Wrapf(ErrNotFound, "address %s not found", hash)
	}

	txs := make([]*ExplorerTx, 0, len(txHashes))
	for i := 0; i < len(txHashes); i++ {
		txHash := txHashes[i]

		tx, err := tangle.GetTransaction(hash)
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
}
