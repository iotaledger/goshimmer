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
13n6HnqiLQVaE2sp8BExM51C2z1BLw7SrFjNAUK439YCC
```

Right after the execution, you will have a new randomly generated address.

## Rand Seed

The rand seed tool is a CLI script a bit more complete than the previous one. It let you generate in a text file a seed (encoded in base58 and base64), its relative identity and its public key. The execution is still simple:

```shell
cd tools/rand-seed
go run main.go
```

The script will generate a `.txt` file containing, for example, the following:

```shell
base64:ri9C8oAT3IPsus2j+IllMbW2B3nOqe4uC56zfr344zY=
base58:CiwjnjMRwEbCGiATWjNsrVptBTNH13AHrVNmG31KK9cy
Identity - base58:BCUnRc6c
Identity - base58:BCUnRc6cv4YVnB3Rw5DDfdsFuVVUW97MyLEBzWxHqfQj
Public Key - base58:Ht9VR8qAgmruDPzsQbak3AJvXcJY6q6Mxyaz4pDicDEw
```

These tools are useful if you have to run, for example, a Docker Private Network and you want to setup the exact nodes running in the network.
