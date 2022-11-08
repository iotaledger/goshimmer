---
description: 'You can use the rand-address and rand-seed tools to generate random seeds and addresses through a simple command.'
keywords:
- address
- seed
- public key
- private key
- generate
- generation
---
# Rand Seed and Rand Address

You can use the [`rand-address`](#rand-address) and [`rand-seed`](#rand-seed) tools to generate addresses and seeds in a single command.

## Rand Address

You can use the `rand-address` tool to generate a random address by running the following command from the `tools/rand-address` directory:

```shell
cd tools/rand-address
go run main.go
```

### Expected Output

The script will output a Base58 string representing the newly generated address, for example:

```shell
13n6HnqiLQVaE2sp8BExM51C2z1BLw7SrFjNAUK439YCC
```

## Rand Seed

You can use the `rand-address` tool to generate a text file with the following:

* A [seed](../tutorials/send_transaction#seed), represented in Base64 and Base58.
* The seed's relative identity, as a Base58 string. 
* The relative identity's public, key in Base58.

```shell
cd tools/rand-seed
go run main.go
```

### Expected Output

The script will generate a `random-seed.txt` file in the current working directory, for example:

```plaintext
base64:ri9C8oAT3IPsus2j+IllMbW2B3nOqe4uC56zfr344zY=
base58:CiwjnjMRwEbCGiATWjNsrVptBTNH13AHrVNmG31KK9cy
Identity - base58:BCUnRc6c
Identity - base58:BCUnRc6cv4YVnB3Rw5DDfdsFuVVUW97MyLEBzWxHqfQj
Public Key - base58:Ht9VR8qAgmruDPzsQbak3AJvXcJY6q6Mxyaz4pDicDEw
```
