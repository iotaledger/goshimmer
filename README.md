# goshimmer

## Run Shimmer

First, you need to [install Go](https://golang.org/doc/install) if it is not already installed on your machine. It is recommended that you use the most recent version of Go.

### Requirements

- gcc: Some packages in this repo might require to be compile by gcc. Windows users can install [MinGW-gcc](http://tdm-gcc.tdragon.net/download). 

To download and install all the package dependencies just run:
```
go get github.com/iotaledger/goshimmer
```

## Build

If you need to develop locally and be able to build by using your local code, i.e., without waiting for pushing your commits on the repo, clone the repository directly inside the `src/github.com/iotaledger/` folder of your `$GOPATH` with the command:

```
git clone git@github.com:iotaledger/goshimmer.git
```

or if you prefer https over ssh

```
git clone https://github.com/iotaledger/goshimmer.git
```

You can build your executable (as well as cross compiling for other architectures) by running the `go build` tool inside the just cloned folder `goshimmer`:

```
go build -o shimmer
```
You can then run by:

```
./shimmer
```

## Docker

To run Shimmer on docker, you must first build the image with
```
docker build -t iotaledger/goshimmer .
```
and then run it with
```
docker run --rm -it -v target/mainnetdb:/root/mainnetdb iotaledger/goshimmer
```
You may replace `target/mainnetdb` with a custom path to the database folder.

To start Shimmer in the background, you can also simply use [Docker Compose](https://docs.docker.com/compose/) by running
```
docker-compose up -d
```
