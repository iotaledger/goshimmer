# Docker entry node

This folder contains the scripts for running a GoShimmer entry node with Docker. 

It builds the Docker image directly from the specified Git tag (such as `v0.1.3`, `master`, `af0ae41d5bfd607123e6cbae271da839a050b220`, ...) and does not depend on the locally checked out version.
The GoShimmer DB is persisted in a named Docker volume.

The entry node exposes the following ports on the host:
- 14626/udp (Autopeering)
- 188/tcp (Analysis Server)
- 80/tcp (Analysis Visualizer)

## How to run

### Create the Docker volume
Before starting an entry node for the specified git tag the first time, a Docker volume needs to be created.
This is only needed once and can be done via the following command: 
```sh
TAG=tag ./create-volume.sh
```
The environment variable `TAG` contains the Git tag of the desired GoShimmer version.
### Run the GoShimmer entry node
To start the actual entry node, run the following: 
```sh
TAG=tag SEED=seed docker-compose up -d --build
```
The optional environment variable `SEED` contains the autopeering seed of the entry node in Base64 encoding.
If `SEED` is not set, the seed will be taken from the DB (if present) in the volume.
As such, `SEED` is only required once when setting or changing the seed of the entry node.

Alternatively to providing the variables in the command, create the file `.env` in the base folder with the following content:
```
# Git tag of the entry node version
TAG=tag

# Autopeering seed used for the entry node
SEED=seed
```
