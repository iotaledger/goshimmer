---
description: The main features in the wallet are requesting tokens from the faucet, sending tokens or assets to addresses, creating digital assets, creating-, transferring- or destroying NFTs, managing NFT owned tokens or assets and delegating tokens or digital assets
image: /img/tutorials/wallet_library/created_nft.png
keywords:
- wallet
- bash
- devnet
- tokens
- digital assets
- creating
- NFT
- managing
- requesting 
---
# Command Line Wallet

This section describes how to use the command line wallet (cli-wallet).

GoShimmer ships with a basic (golang) wallet library so that developers and testers can use it to move tokens around, create digital assets, NFTs or delegate funds.

The cli-wallet is built by using this wallet library to demonstrate the capabilities of the protocol.

The main features in the wallet are:

- [Requesting tokens from the faucet](#requesting-tokens)
- [Sending tokens or assets to addresses](#sending-tokens-and-assets)
- [Creating digital assets](#creating-digital-assets)
- [Creating](#creating-nfts), [transferring](#transferring-nfts) or [destroying](#destroying-nfts) Non-Fungible Tokens (NFTs)
- [Managing NFT owned tokens or assets](#managing-nft-owned-assets)  
- [Delegating tokens or digital assets](#delegating-assets)

:::info
The command line wallet and this tutorial are aimed at a developer audience, you should least be familiar with using a terminal to use it. 
:::

## Initializing the Wallet

1. Download the latest cli-wallet for the system of your choice from the [GoShimmer GitHub Releases](https://github.com/iotaledger/goshimmer/releases) page.
2. If needed, make the downloaded binary executable.  If you are using linux you can run:
   ```shell
   chmod +x <downloaded-binary>
   ```

:::info
For simplicity, we renamed the binary to `cli-wallet` in this tutorial.
:::

You will need to initialize the wallet the first time you start it. This involves generating a secret seed that is used to generate addresses and sign transactions. The wallet will automatically persist the seed in `wallet.dat` after the first run.

You can configure the wallet by creating a `config.json` file in the directory of the executable:

```json
{
	"WebAPI": "http://127.0.0.1:8080",
	"basicAuth": {
	  "enabled": false,
	  "username": "goshimmer",
	  "password": "goshimmer"
	},
	"reuse_addresses": false,
	"faucetPowDifficulty": 25,
	"assetRegistryNetwork": "nectar"
}
```

 - The `WebAPI` tells the wallet which node API to communicate with. Set it to the url of a node API.
 - If the node has basic authentication enabled, you may configure your wallet with a username and password.
 - The `resuse_addresses` option specifies if the wallet should treat addresses as reusable, or whether it should try to spend from any wallet address only once.
 - The `faucetPowDifficulty` option defines the difficulty of the faucet request POW the wallet should do.
 - The `assetRegistryNetwork` option defines which asset registry network to use for pushing/fetching asset metadata to/from the registry. By default, the wallet chooses the `nectar` network.
   
You can initialize your wallet by running the `init` command:

```shell
./cli-wallet init
```

If successful, you'll see the generated seed (encoded in base58) on your screen:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2
GENERATING NEW WALLET ...                                 [DONE]

================================================================
!!!            PLEASE CREATE A BACKUP OF YOUR SEED           !!!
!!!                                                          !!!
!!!       ExzYy6wS2k59dPh19Q9JiAf6z1jyDq1hieDEMmbUzkbE       !!!
!!!                                                          !!!
!!!            PLEASE CREATE A BACKUP OF YOUR SEED           !!!
================================================================

CREATING WALLET STATE FILE (wallet.dat) ...               [DONE]
```

## Requesting Tokens

You can request testnet tokens by executing the `request-funds` command:

```shell
./cli-wallet request-funds
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Requesting funds from faucet ... [PERFORMING POW]          (this can take a while)
Requesting funds from faucet ... [DONE]
```

Once you have executed the `request-funds` command,  you can check the balance of your wallet by running the `balance` command:

```shell
./cli-wallet balance
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Available Token Balances

STATUS  BALANCE                 COLOR                                           TOKEN NAME
------  ---------------         --------------------------------------------    -------------------------
[PEND]  1000000 I               IOTA                                            IOTA
```

Until the network has confirmed the transaction, the status of your token balance will be pending (`[PEND]`). Pending token balances can not be spent, wait until status becomes `[ OK ]`. You can call the `balance` command again to check for status changes.

## Creating Digital Assets

Digital assets are tokens with a special attribute, namely a color. A color is a sequence of 32 bytes represented as a base58 encoded string. The color of a token is derived from the unique transaction that created the asset. Therefore, it is not possible to create assets with the same color in a subsequent transaction.

The transaction "minting" the assets can specify the amount of tokens to be created with the unique color. 

You can create assets with the cli-wallet executing the `create-asset` command:

```shell
./cli-wallet create-asset -name MyUniqueToken -symbol MUT -amount 1000
```

- The `name` flag specifies the name of the asset.
- The `symbol` flag specifies the symbol of the asset.
- The `amount` flag specifies the amount of asset tokens to create.

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Creating 1000 tokens with the color 'HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn' ...   [DONE]
```

If you execute the `balance` command shortly after, you will notice that the wallet balances have changed:

```shell
./cli-wallet balance
IOTA 2.0 DevNet CLI-Wallet 0.2

Available Token Balances

STATUS  BALANCE                 COLOR                                           TOKEN NAME
------  ---------------         --------------------------------------------    -------------------------
[PEND]  999000                  IOTA                                            IOTA
[PEND]  1000                    HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn    MyUniqueToken
```

To create `myUniqueToken`, the wallet has tagged 1000 IOTA tokens with the color `HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn`. The IOTA balance has decreased, but we have received assets in return for the used IOTAs. The created asset tokens behave exactly like other IOTA tokens, they can be transferred without fees to any address.

### Fetching Information of a Digital Asset

In the [previous example](#creating-digital-assets), we have created a digital asset called `MyUniqueToken`. The wallet knows it's name, symbol and initial supply as we provided this input while creating it. The network however does not store this information, it only knows its unique identifier, the assetID (or color).

To help others discover an asset's  attributes, when you create an asset the `cli-wallet` will automatically send this information to a metadata registry service.

When you receive a locally unknown asset to your wallet, it queries this registry service for the metadata. You can also query this metadata yourself by running the `asset-info` command in the wallet:

```shell
./cli-wallet asset-info -id HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Asset Info

PROPERTY                        VALUE
-----------------------         --------------------------------------------
Name                            MyUniqueToken
Symbol                          MUT
AssetID(color)                  HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn
Initial Supply                  1000
Creating Transaction            G7ergf7YzVUSqQMS69jGexYtihbhpsvELEsPHWToYtKj
Network                         test
```

## Sending Tokens and Assets

Funds in IOTA are tied to addresses. Only the owner of the private key behind the address is able to spend (move) the funds, let them be IOTA tokens or digital assets.  In previous sections, you have [requested funds](#requesting-tokens) from the faucet, which actually sent
these tokens to an address provided by your wallet. When you created `MyUniqueToken`, the wallet internally generated a new address
to hold the assets. You may examine the addresses used by the wallet by executing the `address -list` command:

```shell
./cli-wallet address -list
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

INDEX   ADDRESS                                         SPENT
-----   --------------------------------------------    -----
0       19ZD79gRvVzXpQV4QfpY5gefqgrBA4gp11weeyqbY89FK   true
1       1BbywJFGFtDFXpZidmjN39d8cVWUskT2MhbFqSrmVs3qi   false
```

Consequently, when you wish to send tokens, you need to provide an address where to send the tokens to. 

### Simple Send

The `send-funds` command can be used to send IOTA or colored tokens to any address. You can run the following command to see what options you have:

```shell
./cli-wallet send-funds -help
```

Expected output:

```
IOTA 2.0 DevNet CLI-Wallet 0.2

USAGE:
  cli-wallet send-funds [OPTIONS]

OPTIONS:
  -access-mana-id string
        node ID to pledge access mana to
  -amount int
        the amount of tokens that are supposed to be sent
  -color string
        (optional) color of the tokens to transfer (default "IOTA")
  -consensus-mana-id string
        node ID to pledge consensus mana to
  -dest-addr string
        destination address for the transfer
  -fallb-addr string
        (optional) fallback address that can claim back the (unspent) sent funds after fallback deadline
  -fallb-deadline int
        (optional) unix timestamp after which only the fallback address can claim the funds back
  -help
        show this help screen
  -lock-until int
        (optional) unix timestamp until which time the sent funds are locked from spending
```

You can ignore the mana pledge options, as your wallet can derive pledge IDs automatically. The most important options are:

 - `amount` is the amount of token you want to send.
 - `color` is an optional flag to send digital assets with a certain color. When not specified, it defaults to the color of the IOTA token.
 - `dest-addr` is the destination address for the transfer. You will have to set this to the address you wish to transfer tokens to.
 - `fallb-addr` and `fallb-deadline` are optional flags to initiate a conditional transfer. A conditional transfer has a fallback deadline set, after which, only the `fallback-address` can unlock the funds. Before the fallback deadline, it is only the receiver of the funds who can spend the funds. Therefore, conditional transfers have to be claimed by the receiving party before the deadline expires.    
- `lock-until` is an optional flag for a simple time locking mechanism. Before the time lock expires, the funds are locked and can not be spent by the owner.
  
To send 500 `MyUniqueTokens` to the address `1E5Q82XTF5QGyC598br9oCj71cREyjD1CGUk2gmaJaFQt`, you have to tell the wallet that `MyUniqueTokens` are of color `HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn`, as shown in the following command:

```shell
./cli-wallet send-funds -amount 500 -color HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn -dest-addr 1E5Q82XTF5QGyC598br9oCj71cREyjD1CGUk2gmaJaFQt
```

### Time Locked Sending

If you don't want the receiver to be able to spend the tokens you have sent right away, you should execute the `send-funds` command with the `-lock-until` flag. The `-lock-until` flag expects a unix timestamp. For example, on linux, you can get a unix timestamp 7 days in the future by executing:

```shell
date -d "+7 days" +%s
1621426409
```

Once you have a unix timestamp, you can execute the transfer by running:

```shell
$ ./cli-wallet send-funds -amount 500 -dest-addr 1E5Q82XTF5QGyC598br9oCj71cREyjD1CGUk2gmaJaFQt -lock-until 1621426409
```

### Conditional Sending

You have the option to specify a fallback unlocking mechanism on the tokens that you send. If the recipient doesn't claim the funds before the fallback deadline you specify expires, the fallback address can essentially take back the tokens.

If you want to send some IOTAs, but if the receiver doesn't claim them for a week you want to
have them back you should use the `-fallb-addr` and `-fallb-dealine` flags when executing the `send-funds` command.

If you want to use your own wallet as fallback address, you can get your wallet's receive by running `./cli-wallet address -receive`:

```shell
./cli-wallet address -receive
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Latest Receive Address: 17KoEZbWoBLRjBsb6oSyrSKVVqd7DVdHUWpxfBFbHaMSm
```

Once you have the desired fallback address, you can execute a transfer with a fallback address by running:

```shell
./cli-wallet send-funds -amount 500 -dest-addr 1E5Q82XTF5QGyC598br9oCj71cREyjD1CGUk2gmaJaFQt \
-fallb-addr 17KoEZbWoBLRjBsb6oSyrSKVVqd7DVdHUWpxfBFbHaMSm --fallb-deadline 1621426409
```

When you receive conditional funds, they will be displayed on the balance page in the wallet:

```shell
./cli-wallet balance
IOTA 2.0 DevNet CLI-Wallet 0.2

Available Token Balances

STATUS  BALANCE                 COLOR                                           TOKEN NAME
------  ---------------         --------------------------------------------    -------------------------
[ OK ]  500 I                   IOTA                                            IOTA

Conditional Token Balances - execute `claim-conditional` command to sweep these funds into wallet

STATUS  OWNED UNTIL                     BALANCE                 COLOR                                           TOKEN NAME
------  ------------------------------  ---------------         --------------------------------------------    -------------------------
[ OK ]  2021-05-19 14:13:29 +0200 CEST  500 I                   IOTA                                            IOTA
```

As the output suggests, you need to execute the `claim-conditional` command to claim these funds:

```shell
./cli-wallet claim-conditional
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Claiming conditionally owned funds... [DONE]
```

After claiming the funds, you can see your balance has been updated by running:

```shell
./cli-wallet balance
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Available Token Balances

STATUS  BALANCE                 COLOR                                           TOKEN NAME
------  ---------------         --------------------------------------------    -------------------------
[PEND]  500                     IOTA                                            IOTA
```

## Creating NFTs

NFTs are non-fungible tokens that have unique properties. In IOTA, NFTs are represented as non-forkable, uniquely identifiable outputs. When you spend an NFT, the transaction will only be considered valid if it satisfies the constraints defined in the outputs. For example, the immutable data attached to the output can not change. Therefore, we can create an NFT and record immutable metadata in its output.

You can list the option for the `create-nft` command by running `./cli-wallet create-nft -help`:

```shell
./cli-wallet create-nft -help
IOTA 2.0 DevNet CLI-Wallet 0.2

USAGE:
  cli-wallet create-nft [OPTIONS]

OPTIONS:
  -access-mana-id string
        node ID to pledge access mana to
  -color string
        color of the tokens that should be deposited into the nft upon creation (on top of the minimum required) (default "IOTA")
  -consensus-mana-id string
        node ID to pledge consensus mana to
  -help
        show this help screen
  -immutable-data string
        path to the file containing the immutable data that shall be attached to the nft
  -initial-amount int
        the amount of tokens that should be deposited into the nft upon creation (on top of the minimum required)
```

None of the flags are strictly required to mint an NFT, so we could just execute the command as it is. However, in most cases, you will want to attach immutable metadata to it, which is only possible during creation. Each NFT must have some IOTAs backing it (locked into its output) to prevent bloating the ledger database. Currently, the minimum requirement is 100 IOTA tokens, but bear in mind that it might change in the future. On top of the minimum required amount IOTAs, you can lock any additional funds into the NFT. You can use the `-initial-amount` and `-color` flags to do so.

To attach immutable data to the NFT, you should define a path to a file that holds the metadata. The wallet will read the byte content of the file, and attach it to the NFT. Currently, the maximum allowed metadata file size is 4 kilobytes. You should use the `-immutable-data` flag to specify a path to a file that holds the metadata.

For example, you can create a `nft_metadata.json` file in the directory of the cli-wallet with the following content:

```json
{
  "title": "Asset Metadata",
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "description": "MyFirstNFT"
    },
    "description": {
      "type": "string",
      "description": "My very first NFT that has this metadata attached to it."
    },
    "image": {
      "type": "string",
      "description": "<uri to resource holding the content of my first NFT>"
    }
  }
}
```

The above JSON file is just a template, you can define any binary data that fits the size limit to be attached to the NFT.

After you have created your data file, you can create the NFT by executing:

```shell
./cli-wallet create-nft -immutable-data nft_metadata.json
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Created NFT with ID:  gSfeBrWp1HwDLwSL7rt1qEMM59YBFZ4iBgAqHuqaQHo5
Creating NFT ... [DONE]
```

The created NFT's unique identifier is `gSfeBrWp1HwDLwSL7rt1qEMM59YBFZ4iBgAqHuqaQHo5`, which is also a valid IOTA address. Navigate to a node dashboard/explorer and search for the address. On a node dashboard, you would see something like this:

[![Created NFT Example](/img/tutorials/wallet_library/created_nft.png)](/img/tutorials/wallet_library/created_nft.png)

The immutable data field contains the attached binary metadata (encoded in base64 in the node dashboard).

The NFT is also displayed on the balance page of the cli-wallet:

```shell
./cli-wallet balance
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Available Token Balances

STATUS  BALANCE                 COLOR                                           TOKEN NAME
------  ---------------         --------------------------------------------    -------------------------
[ OK ]  500 MUT                 HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn    MyUniqueToken
[ OK ]  996200 I                IOTA                                            IOTA

Owned NFTs (Governance Controlled Aliases)

STATUS  NFT ID (ALIAS ID)                               BALANCE                 COLOR                                           TOKEN NAME
------  --------------------------------------------    ---------------         --------------------------------------------    -------------------------
[ OK ]  gSfeBrWp1HwDLwSL7rt1qEMM59YBFZ4iBgAqHuqaQHo5    100                     IOTA                                            IOTA
```

## Transferring NFTs

You can use the `transfer-nft` command to send NFT.  You can run the `transfer-nft` command with the `-help` flagTo view the available options.

```shell
./cli-wallet transfer-nft -help
IOTA 2.0 DevNet CLI-Wallet 0.2

USAGE:
  cli-wallet transfer-nft [OPTIONS]

OPTIONS:
  -access-mana-id string
        node ID to pledge access mana to
  -consensus-mana-id string
        node ID to pledge consensus mana to
  -dest-addr string
        destination address for the transfer
  -help
        show this help screen
  -id string
        unique identifier of the nft that should be transferred
  -reset-delegation
        defines whether to reset the delegation status of the alias being transferred
  -reset-state-addr
        defines whether to set the state address to dest-addr
```

There are 2 mandatory flags that you will need to provide for a valid transfer: `-id` and `-dest-addr`. `-id` is unique identifier of the NFT that you wish to transfer, `-dest-addr` is the destination address.

The following command will send some of the `MyUniqueTokens` created in the previous example to `1E5Q82XTF5QGyC598br9oCj71cREyjD1CGUk2gmaJaFQt`: 

```shell
./cli-wallet transfer-nft -id gSfeBrWp1HwDLwSL7rt1qEMM59YBFZ4iBgAqHuqaQHo5 -dest-addr 1E5Q82XTF5QGyC598br9oCj71cREyjD1CGUk2gmaJaFQt
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Transferring NFT... [DONE]
```

## Destroying NFTs

The owner of an NFT has the ability to destroy it. When an NFT is destroyed, all of its balance will be transferred to the NFT's current owner, and the alias output representing the NFT will be spent without creating a corresponding next alias output.


You can use the `destroy-nft` command to destroy a NFT.  You can run the `destroy-nft` command with the `-help` flag to view the available options.

```shell
./cli-wallet destroy-nft -help
IOTA 2.0 DevNet CLI-Wallet 0.2

USAGE:
  cli-wallet destroy-nft [OPTIONS]

OPTIONS:
  -access-mana-id string
        node ID to pledge access mana to
  -consensus-mana-id string
        node ID to pledge consensus mana to
  -help
        show this help screen
  -id string
        unique identifier of the nft that should be destroyed

```

The following example shows how to create a NFT, and destroy it right after.

1. Create the NFT
   ```shell
   ./cli-wallet create-nft
   ```
   Expected output:
   ```
   IOTA 2.0 DevNet CLI-Wallet 0.2
   
   Created NFT with ID:  bdrvyKvaE6CZUEbdRDK57oBCRb2SLUyE8padFGxrV3zg
   Creating NFT ... [DONE]
   ```
2. Check the balance page shows that the NFT status is `OK`:
   ```shell
   ./cli-wallet balance
   ```
   
   Expected output:

   ```shell
   IOTA 2.0 DevNet CLI-Wallet 0.2
   
   Available Token Balances
   
   STATUS  BALANCE                 COLOR                                           TOKEN NAME
   ------  ---------------         --------------------------------------------    -------------------------
   [ OK ]  500 MUT                 HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn    MyUniqueToken
   [ OK ]  996100 I                IOTA                                            IOTA
   
   Owned NFTs (Governance Controlled Aliases)
   
   STATUS  NFT ID (ALIAS ID)                               BALANCE                 COLOR                                           TOKEN NAME
   ------  --------------------------------------------    ---------------         --------------------------------------------    -------------------------
   [ OK ]  bdrvyKvaE6CZUEbdRDK57oBCRb2SLUyE8padFGxrV3zg    100                     IOTA                                            IOTA
   ```
   
3. Destroy the nft:

   ```shell
   ./cli-wallet destroy-nft -id bdrvyKvaE6CZUEbdRDK57oBCRb2SLUyE8padFGxrV3zg
   ```
   
   Expected output:
   
   ```
   IOTA 2.0 DevNet CLI-Wallet 0.2
   
   Destroying NFT... [DONE]
   ```

## Managing NFT Owned Assets

An NFT is not only a valid IOTA address via its NFT ID, but it is also stored as an output in the ledger. Therefore, the NFT is not only capable of receiving funds to its address, but the owner can directly manage the funds held in the NFT output.

 - The owner may deposit assets into an NFT, or withdraw assets from there,  essentially using it as a standalone wallet.
 - Other users in the network can send any asset to the NFT address, that will be owned by the NFT. The owner might choose to deposit those funds into the NFT, or sweep them into their own wallet.
   
### Deposit Assets Into Owned NFT

You can use the `deposit-to-nft` command to transfer tokens to a NFT. You can run the `deposit-to-nft` command with the `-help` flag to view the available options.

```shell
./cli-wallet deposit-to-nft -help
IOTA 2.0 DevNet CLI-Wallet 0.2

USAGE:
  cli-wallet deposit-to-nft [OPTIONS]

OPTIONS:
  -access-mana-id string
        node ID to pledge access mana to
  -amount int
        the amount of tokens that are supposed to be deposited
  -color string
        color of funds to deposit (default "IOTA")
  -consensus-mana-id string
        node ID to pledge consensus mana to
  -help
        show this help screen
  -id string
        unique identifier of the nft to deposit to
```

To deposit some previously created `MyUniqueTokens` into the NFT, we need to specify the following flags:

 - `-id` the NFT ID to deposit to.
 - `-amount` amount of assets to deposit.
 - `-color` asset color to deposit.

You can check your balance before the transfer by running:

```shell
./cli-wallet balance
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Available Token Balances

STATUS  BALANCE                 COLOR                                           TOKEN NAME
------  ---------------         --------------------------------------------    -------------------------
[ OK ]  996300 I                IOTA                                            IOTA
[ OK ]  500 MUT                 HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn    MyUniqueToken

Owned NFTs (Governance Controlled Aliases)

STATUS  NFT ID (ALIAS ID)                               BALANCE                 COLOR                                           TOKEN NAME
------  --------------------------------------------    ---------------         --------------------------------------------    -------------------------
[ OK ]  f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg    100                     IOTA                                            IOTA
```

You can run the following command to deposit 500 `MyUniqueTokens` to the nft with id `f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg`:

```shell
./cli-wallet deposit-to-nft -id f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg -amount 500 -color HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2
Depositing funds into NFT ... [DONE]
```

After the transfer is successful, you can recheck your balance, and it should show your NFT now has 500 `MyUniqueTokens`:

```shell
./cli-wallet balance
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Available Token Balances

STATUS  BALANCE                 COLOR                                           TOKEN NAME
------  ---------------         --------------------------------------------    -------------------------
[ OK ]  996300 I                IOTA                                            IOTA

Owned NFTs (Governance Controlled Aliases)

STATUS  NFT ID (ALIAS ID)                               BALANCE                 COLOR                                           TOKEN NAME
------  --------------------------------------------    ---------------         --------------------------------------------    -------------------------
[ OK ]  f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg    100                     IOTA                                            IOTA
                                                        500                     HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn    MyUniqueToken
```

### Withdrawing Assets From NFT

You can use the `withdraw-from-nft` command to withdraw tokens from a NFT.  If the withdrawal leaves less than the minimum required funds in the NFT, the transaction will fail. You can run the `withdraw-from-nft` command with the `-help` flag to view the available options.

```shell
./cli-wallet withdraw-from-nft -help
IOTA 2.0 DevNet CLI-Wallet 0.2

USAGE:
  cli-wallet withdraw-from-nft [OPTIONS]

OPTIONS:
  -access-mana-id string
        node ID to pledge access mana to
  -amount int
        the amount of tokens that are supposed to be withdrew
  -color string
        color of funds to withdraw (default "IOTA")
  -consensus-mana-id string
        node ID to pledge consensus mana to
  -dest-addr string
        (optional) address to send the withdrew tokens to
  -help
        show this help screen
  -id string
        unique identifier of the nft to withdraw from
```

You can execute the following command to withdraw the previously deposited `MyUniqueTokens`:

```shell
./cli-wallet withdraw-from-nft -id f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg -amount 500 -color HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Withdrawing funds from NFT... [DONE]
```

Once the transaction has been confirmed, you will see the updated balance:

```shell
./cli-wallet balance
```

Expected output:

```shell
IOTA 2.0 DevNet CLI-Wallet 0.2

Available Token Balances

STATUS  BALANCE                 COLOR                                           TOKEN NAME
------  ---------------         --------------------------------------------    -------------------------
[ OK ]  500 MUT                 HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn    MyUniqueToken
[ OK ]  996300 I                IOTA                                            IOTA

Owned NFTs (Governance Controlled Aliases)

STATUS  NFT ID (ALIAS ID)                               BALANCE                 COLOR                                           TOKEN NAME
------  --------------------------------------------    ---------------         --------------------------------------------    -------------------------
[ OK ]  f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg    100                     IOTA                                            IOTA
```

### Sweep NFT Owned Funds

You can use the `sweep-nft-owned-fun` command to collect all funds currently stored in a NFT. You can run the `sweep-nft-owned-fun` command with the `-help` flag to view the available options.

```shell
./cli-wallet sweep-nft-owned-funds -help
IOTA 2.0 DevNet CLI-Wallet 0.2

USAGE:
  cli-wallet sweep-nft-owned-funds [OPTIONS]

OPTIONS:
  -access-mana-id string
        node ID to pledge access mana to
  -consensus-mana-id string
        node ID to pledge consensus mana to
  -help
        show this help screen
  -id string
        unique identifier of the nft that should be checked for outputs with funds
  -to string
        optional address where to sweep
```

The only mandatory flag is `-id`, as it specifies which NFT ID (address) to scan for funds.

In the following example, a sender has sent token to our NFT `f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg` with a normal `send-funds` command:

```shell
./senders-wallet send-funds -amount 1000000 -dest-addr f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg
```

You can execute the `sweep-nft-owned-funds` command to transfer these funds into our wallet:

```shell
./cli-wallet sweep-nft-owned-funds -id f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg
```
Expected output:

```
IOTA 2.0 DevNet CLI-Wallet 0.2

Sweeping NFT owned funds... [DONE]
```

If you check the balance, it should be updated.  So the wallet contains 1 MI more:

```shell
./cli-wallet balance
```

Expected output:

```
IOTA 2.0 DevNet CLI-Wallet 0.2

Available Token Balances

STATUS  BALANCE                 COLOR                                           TOKEN NAME
------  ---------------         --------------------------------------------    -------------------------
[ OK ]  500 MUT                 HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn    MyUniqueToken
[ OK ]  1996300 I               IOTA                                            IOTA

Owned NFTs (Governance Controlled Aliases)

STATUS  NFT ID (ALIAS ID)                               BALANCE                 COLOR                                           TOKEN NAME
------  --------------------------------------------    ---------------         --------------------------------------------    -------------------------
[ OK ]  f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg    100                     IOTA                                            IOTA
```

### Sweep NFT Owned NFTs

NFTs can own other NFTs, that in turn can own other NFTs and so on... wow, NFTception!
Let's say your friend created an NFT, and transferred it to your NFT's ID `f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg`.

```shell
./your-friends-wallet create-nft
```

Expected output:

```
IOTA 2.0 DevNet CLI-Wallet 0.2

Created NFT with ID:  faf9tkdBfcTv2AgPm3Zt8duX4iUGKjqbEyrdBYsUb2hi
Creating NFT ... [DONE]
```
```
./your-friends-wallet transfer-nft -id faf9tkdBfcTv2AgPm3Zt8duX4iUGKjqbEyrdBYsUb2hi -dest-addr f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg
```
```
IOTA 2.0 DevNet CLI-Wallet 0.2

Transferring NFT... [DONE]
```

Your NFT `f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg` now owns NFT `faf9tkdBfcTv2AgPm3Zt8duX4iUGKjqbEyrdBYsUb2hi`.
To sweep the owned NFT into your wallet, execute the `sweep-nft-owned-nft` command:

```shell
./cli-wallet sweep-nft-owned-nfts -help
IOTA 2.0 DevNet CLI-Wallet 0.2

USAGE:
  cli-wallet sweep-nft-owned-nfts [OPTIONS]

OPTIONS:
  -access-mana-id string
        node ID to pledge access mana to
  -consensus-mana-id string
        node ID to pledge consensus mana to
  -help
        show this help screen
  -id string
        unique identifier of the nft that should be checked for owning other nfts
  -to string
        optional address where to sweep
```

All you need to specify is the `-id` of your NFT that you would like to check for owned NFTs:

```shell
./cli-wallet sweep-nft-owned-nfts -id f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg
```

```
IOTA 2.0 DevNet CLI-Wallet 0.2

Swept NFT faf9tkdBfcTv2AgPm3Zt8duX4iUGKjqbEyrdBYsUb2hi into the wallet
Sweeping NFT owned NFTs... [DONE]
```
That's it, your wallet owns `faf9tkdBfcTv2AgPm3Zt8duX4iUGKjqbEyrdBYsUb2hi` now. If this NFT owned other funds or NFTs,
you would be able to sweep them into your wallet just like you did for `f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg`.

```shell
./cli-wallet balance
```

```
IOTA 2.0 DevNet CLI-Wallet 0.2

Available Token Balances

STATUS  BALANCE                 COLOR                                           TOKEN NAME
------  ---------------         --------------------------------------------    -------------------------
[ OK ]  1996300 I               IOTA                                            IOTA
[ OK ]  500 MUT                 HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn    MyUniqueToken

Owned NFTs (Governance Controlled Aliases)

STATUS  NFT ID (ALIAS ID)                               BALANCE                 COLOR                                           TOKEN NAME
------  --------------------------------------------    ---------------         --------------------------------------------    -------------------------
[ OK ]  f1BW8jcdDn3staviCVbVz54NqVwsshb5gpNLqY6Rrgrg    100                     IOTA                                            IOTA
[ OK ]  faf9tkdBfcTv2AgPm3Zt8duX4iUGKjqbEyrdBYsUb2hi    100                     IOTA                                            IOTA
```

## Delegating Assets

The primary use case of fund delegation in Coordicide is to enable refreshing a node's access mana without requiring
the use of a master key that has full control over the funds. A delegated key can not spend the funds, but can
"refresh" the outputs holding the funds in a transaction that can pledge mana to any arbitrary nodes.

A token holder can therefore keep their funds in secure cold storage, while delegating them to a node or third party
to utilize the mana generated by the funds. Assuming there is demand for access mana in the network, the holder of the
assets can then sell the generated mana to realize return on their assets.

Delegating funds via the cli-wallet is rather simple: you just need to execute the `delegate-funds` command. By default,
the cli-wallet will delegate funds to the node that the wallet is connected to, unless you specify a delegation
address via the `-del-addr` flag.
specify a valid IOTA address where to delegate to.

```shell
./cli-wallet delegate-funds -help
IOTA 2.0 DevNet CLI-Wallet 0.2

USAGE:
  cli-wallet delegate-funds [OPTIONS]

OPTIONS:
  -access-mana-id string
        node ID to pledge access mana to
  -amount int
        the amount of tokens that should be delegated
  -color string
        color of the tokens that should delegated (default "IOTA")
  -consensus-mana-id string
        node ID to pledge consensus mana to
  -del-addr string
        address to delegate funds to. when omitted, wallet delegates to the node it is connected to
  -help
        show this help screen
  -until int
        unix timestamp until which the delegated funds are timelocked
```

 - Mandatory parameter is only the `-amount`.
 - Use the `-del-addr` flag to delegate to arbitrary address.
 - You may specify a delegation deadline via the `-until` flag. If this is set, the delegated party can not unlock
   the funds for refreshing mana after the deadline expired, but the neither can the owner reclaim the funds before
   that. If the `-until` flag is omitted, the delegation is open-ended, the owner can reclaim the delegated funds at
   any time.
 - You can specify a certain asset to be delegated (`-color`), default is IOTA.

Let's delegate some funds to an address provided by a node in the network, `1EqJf5K1LJ6bVMCrxxxdZ6VNYoBTvEoXgxnbLJe7aqajc`:

```shell
./cli-wallet delegate-funds -amount 1000000 -del-addr 1EqJf5K1LJ6bVMCrxxxdZ6VNYoBTvEoXgxnbLJe7aqajc
```

```
IOTA 2.0 DevNet CLI-Wallet 0.2

Delegating to address 1EqJf5K1LJ6bVMCrxxxdZ6VNYoBTvEoXgxnbLJe7aqajc
Delegation ID is:  tGoTKjt2y277ssKax9stsZXfLGdf8bPj3TZFaUDcAEwK
Delegating funds... [DONE]
```

If we omitted the `-del-addr` flag and its value, the wallet would have asked the node it is connected to, to provide
a delegation address. You can get this delegation address yourself as well by running the `server-status` command in
the wallet, or querying the `/info` endpoint of a node through the webapi.

```shell
./cli-wallet server-status
```

```
IOTA 2.0 DevNet CLI-Wallet 0.2

Server ID:  2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5
Server Synced:  true
Server Version:  v0.5.9
Delegation Address:  1HG9Z5NSiWTmT1HG65JLmn1jxQj7xUcVppKKi2vHAZLmr
```

By running the `balance` command, we can see the delegated funds:

```shell
./cli-wallet balance
```

```
IOTA 2.0 DevNet CLI-Wallet 0.2

Available Token Balances

STATUS  BALANCE                 COLOR                                           TOKEN NAME
------  ---------------         --------------------------------------------    -------------------------
[ OK ]  500 MUT                 HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn    MyUniqueToken
[ OK ]  996500 I                IOTA                                            IOTA

Delegated Funds

STATUS  DELEGATION ID (ALIAS ID)                        BALANCE                 COLOR                                           TOKEN NAME
------  --------------------------------------------    ---------------         --------------------------------------------    -------------------------
[ OK ]  tGoTKjt2y277ssKax9stsZXfLGdf8bPj3TZFaUDcAEwK    1000000                 IOTA                                            IOTA
```

To be able to reclaim the delegated funds, we will need the delegation ID of the delegated funds.

## Reclaiming Delegated Assets

To reclaim delegated funds, you have to tell the cli-wallet the delegation ID that is displayed on the balance page.
Use the `reclaim-delegated` command once you got the delegation ID:

```shell
 ./cli-wallet reclaim-delegated -help
IOTA 2.0 DevNet CLI-Wallet 0.2

USAGE:
  cli-wallet reclaim-delegated [OPTIONS]

OPTIONS:
  -access-mana-id string
        node ID to pledge access mana to
  -consensus-mana-id string
        node ID to pledge consensus mana to
  -help
        show this help screen
  -id string
        delegation ID that should be reclaimed
  -to-addr string
        optional address where to send reclaimed funds, wallet receive address by default
```

To reclaim the funds delegated in the previous section, simply run:

```shell
./cli-wallet reclaim-delegated -id tGoTKjt2y277ssKax9stsZXfLGdf8bPj3TZFaUDcAEwK
```

```
IOTA 2.0 DevNet CLI-Wallet 0.2

Reclaimed delegation ID is:  tGoTKjt2y277ssKax9stsZXfLGdf8bPj3TZFaUDcAEwK
Reclaiming delegated fund... [DONE]
```
The balance should appear in the `Available Balances` section of the balance page:

```shell
./cli-wallet balance
```

```
IOTA 2.0 DevNet CLI-Wallet 0.2

Available Token Balances

STATUS  BALANCE                 COLOR                                           TOKEN NAME
------  ---------------         --------------------------------------------    -------------------------
[ OK ]  500 MUT                 HJdkZkn6MKda9fNuXFQZ8Dzdzu1wvuSUQp8QX1AMH4wn    MyUniqueToken
[ OK ]  1996500 I               IOTA                                            IOTA
```

## Common Flags

As you may have noticed, there are some universal flags in many commands, namely:
 - `-help` that brings up the command usage and help information,
 - `access-mana-id` that is the nodeID to which the transaction should pledge access mana to, and
 - `consensus-mana-id` that is the nodeID to which the transaction should pledge consensus mana to.

The latter teo are determined by default by your wallet depending on which node you connect it to. However, if that node
doesn't filter user submitted transactions based on the mana pledge IDs, you are free to define which node to pledge
mana to.

## Command Reference

### balance
Show the balances held by this wallet.
### send-funds
Initiate a transfer of tokens or assets (funds).
### consolidate-funds
Consolidate all available funds to one wallet address.
### claim-conditional
Claim (move) conditionally owned funds into the wallet.
### request-funds
Request funds from the testnet-faucet.
### create-asset
Create an asset in the form of colored coins.
### delegate-funds
Delegate funds to an address.
### reclaim-delegated
Reclaim previously delegated funds.
### create-nft
Create an NFT as an unforkable alias output.
### transfer-nft
Transfer the ownership of an NFT.
### destroy-nft
Destroy an NFT.
### deposit-to-nft
Deposit funds into an NFT.
### withdraw-from-nft
Withdraw funds from an NFT.
### sweep-nft-owned-funds
Sweep all available funds owned by NFT into the wallet.
### sweep-nft-owned-nfts
weep all available NFTs owned by NFT into the wallet.
### address
Start the address manager of this wallet.
### init
Generate a new wallet using a random seed.
### server-status
Display the server status.
### pending-mana
Display current pending mana of all outputs in the wallet grouped by address.
### pledge-id
Query nodeIDs accepted as pledge IDs in transaction by the node (server).
### help
Display this help screen.
