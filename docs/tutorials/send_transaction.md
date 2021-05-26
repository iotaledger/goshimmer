# How to send transaction
To send a transaction, you have to first construct the transaction and get its bytes.

## Build transaction
Constructs a new `ledgerstate.Transaction`.

```go
import (
	"fmt"
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/client"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
)

func buildTransaction() (tx *ledgerstate.Transaction, err error) {
	// node to pledge access mana.
	accessManaPledgeIDBase58 := "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5"
	accessManaPledgeID, err := mana.IDFromStr(accessManaPledgeIDBase58)
	if err != nil {
		return
	}

	// node to pledge consensus mana.
	consensusManaPledgeIDBase58 := "1HzrfXXWhaKbENGadwEnAiEKkQ2Gquo26maDNTMFvLdE3"
	consensusManaPledgeID, err := mana.IDFromStr(consensusManaPledgeIDBase58)
	if err != nil {
		return
	}
     
        /**
        N.B to pledge mana to the node issuing the transaction, use empty pledgeIDs.
        emptyID := identity.ID{}
        accessManaPledgeID, consensusManaPledgeID := emptyID, emptyID
        **/      

	// destination address.
	destAddressBase58 := "18iBVua17jndUDyfg1Lzgz9gJgEdeo9Dh32nEzwC1R7iy"
	destAddress, err := ledgerstate.AddressFromBase58EncodedString(destAddressBase58)
	if err != nil {
		return
	}

	// output to consume.
	outputIDBase58 := "1B9kpcFhJzStQZzmoLjd6gLmiFJd7F72XBLHbWTQrfkFY"
	out, err := ledgerstate.OutputIDFromBase58(outputIDBase58)
	if err != nil {
		return
	}
	inputs := ledgerstate.NewInputs(ledgerstate.NewUTXOInput(out))

	// UTXO output.
	output := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
		ledgerstate.ColorIOTA: uint64(1337),
	}), destAddress)
	outputs := ledgerstate.NewOutputs(output)

	// build tx essence.
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), accessManaPledgeID, consensusManaPledgeID, inputs, outputs)

	// sign.
	seed := walletseed.NewSeed([]byte("your_seed"))
	kp := seed.KeyPair(0)
	sig := ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes()))
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(sig)

	// build tx.
	tx = ledgerstate.NewTransaction(txEssence, ledgerstate.UnlockBlocks{unlockBlock})
	return
}
```

## Post the transaction

There are 2 available options to post the created transaction.

 - Goshimmer client lib
 - Web API
 
### Post via client lib
```go
func postTransactionViaClientLib() (res string , err error) {
	// connect to goshimmer node
	goshimmerClient := client.NewGoShimmerAPI("http://127.0.0.1:8080", client.WithHTTPClient(http.Client{Timeout: 60 * time.Second}))

	// build tx from previous step
	tx, err := buildTransaction()
	if err != nil {
		return
	}

	// send the tx payload.
	res, err = goshimmerClient.PostTransaction(tx.Bytes())
	if err != nil {
		return
	}
	return
}
```

### Post via web API
First, get the transaction bytes.
```go
	// build tx from previous step
	tx, err := buildTransaction()
	if err != nil {
		return
	}
    bytes := tx.Bytes()
    
    // print bytes
    fmt.Println(string(bytes))
```

Then, post the bytes.

```shell script
curl --location --request POST 'http://localhost:8080/ledgerstate/transactions' \
--header 'Content-Type: application/json' \
--data-raw '{
    "tx_bytes": "bytes..."
}'
```