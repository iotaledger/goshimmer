# goshimmer

[![Build Status](https://travis-ci.org/iotaledger/goshimmer.svg?branch=master)](https://travis-ci.org/iotaledger/goshimmer)

## Run Shimmer

First, you need to [install Go](https://golang.org/doc/install) if it is not already installed on your machine. It is recommended that you use the most recent version of Go.

### Requirements

- gcc: Some packages in this repo might require to be compiled by gcc. Windows users can install [MinGW-gcc](http://tdm-gcc.tdragon.net/download). 


## Build

If you need to develop locally and be able to build by using your local code, i.e., without waiting for pushing your commits on the repo, clone the repository directly inside the `src/github.com/iotaledger/` folder of your `$GOPATH` with the command:

```
git clone git@github.com:iotaledger/goshimmer.git
```

or if you prefer https over ssh

```
git clone https://github.com/iotaledger/goshimmer.git
```

Verify that you have installed the minimal required go version (1.12.7):
```
go version
```

You can build your executable (as well as cross compiling for other architectures) by running the `go build` tool inside the just cloned folder `goshimmer`:

```
go build -o shimmer
```

On Windows:
```
ren shimmer shimmer.exe
```

You can then run by:

Linux
```
./shimmer
```

Windows
```
shimmer
```

## Docker

To run Shimmer on docker, you must first build the image with
```
docker build -t iotaledger/goshimmer .
```
and then run it with
```
docker run --rm -it -v "$(pwd)/mainnetdb:/app/mainnetdb" iotaledger/goshimmer
```
You may replace `$(pwd)/mainnetdb` with a custom path to the database folder.

To start Shimmer in the background, you can also simply use [Docker Compose](https://docs.docker.com/compose/) by running
```
docker-compose up -d
```

### Install Glumb visualizer

Install both the Glumb visualizer and socket.io client lib within the root folder/where the binary is located:
```bash
git clone https://github.com/glumb/IOTAtangle.git
// only this version seems to be stable
cd IOTAtangle && git reset --hard 07bba77a296a2d06277cdae56aa963abeeb5f66e 
cd ../
git clone https://github.com/socketio/socket.io-client.git
```
