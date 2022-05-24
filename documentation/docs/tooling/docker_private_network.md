---
description: GoShimmer provides a tool at `tools/docker-network` with which a local test network can be set up locally with docker.
image: /img/tooling/docker-network.png
keywords:
- docker 
- docker network
- dashboard
- web api
- host system
- port
- docker compose
- analysis dashboard
---
# Docker Private Network

We provide a tool at `tools/docker-network` with which a local test network can be set up locally with docker. 
 
[![Docker network](/img/tooling/docker-network.png "Docker network")](/img/tooling/docker-network.png)

## How to Use the Tool

In the docker network run for example
```shell
./run.sh 5 1 1
```

The command `./run.sh` spins up a GoShimmer network within Docker as schematically shown in the figure above. The first integer input defines the number of `peer_replicas` `N`. The second argument is optional for activating the Grafana dashboard, where 
* default (no argument) or 0: Grafana disabled
* 1: Grafana enabled

More details on how to set up the dashboard can be found [here](../tutorials/setup.md).

The third argument is optional for activating a dRNG committee, where
* default (no argument) or 0: dRNG disabled
* 1: dRNG enabled

The peers can communicate freely within the Docker network 
while the analysis and visualizer dashboard, as well as the `peer_master's` dashboard and web API are reachable from the host system on the respective ports.

The settings for the different containers (`peer_master`, `peer_replica`) can be modified in `docker-compose.yml`.

## How to Use as Development Tool

Using a standalone throwaway Docker network can be really helpful as a development tool. 

Prerequisites: 
- Docker 17.12.0+
- Docker compose: file format 3.5

Reachable from the host system
- `peer_master's` analysis dashboard (autopeering visualizer): http://localhost:9000
- `peer_master's` web API: http: http://localhost:8080
- `faucet's` dashboard: http: http://localhost:8081
<!-- Running the dashboard and analysis dashboard on the same container causes the dashboard to malfunction -->

It is therefore possible to send messages to the local network via the `peer_master`. Log messages of a specific containter can be followed via 
```shell
docker logs --follow CONTAINERNAME
```

## Snapshot Tool

A snapshot tool is provided in the tools folder. The snapshot file that is created must be moved into the `integration-tests/assets` folder. There, rename and replace the existing bin file (`7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih.bin`). After restarting the docker network the snapshot file will be loaded.

Docker Compose uses the `SNAPSHOT_FILE` environment variable to determine the location of the snapshot. Once you have a new snapshot you can simply set `SNAPSHOT_FILE` to the location of your new snapshot and Docker Compose will use your snapshot the next time you run `docker-compose up`.

## How to Use Message Approval Check Tool

`get_approval_csv.sh` script helps you conveniently trigger the message approval checks on all nodes in the docker
network, and gather their results in the `csv` folder.

Once the network is up and running, execute the script:

```shell
./get_approval_csv.sh
```
Example output:
```
Triggering approval analysis on peer_master and 20 replicas...
Triggering approval analysis on peer_master and 20 replicas... DONE
Copying csv files from peer_master and 20 replicas...
Copying csv files from peer_master and 20 replicas... DONE
Copied files are located at ./csv
```
The exported csv files are timestamped to the date of request.
```
csv
├── 210120_16_34_14-docker-network_peer_replica_10.csv
├── 210120_16_34_14-docker-network_peer_replica_11.csv
├── 210120_16_34_14-docker-network_peer_replica_12.csv
...
```
Note, that the record length of the files might differ, since the approval check execution time of the nodes might differ.

## Spammer Tool

The Spammer tool lets you add messages to the tangle when running GoShimmer in a Docker network.
In order to start the spammer, you need to send GET requests to a `/spammer` API endpoint with the following parameters:
* `cmd` - one of two possible values: `start` and `stop`.
* `unit` - Either `mps` or `mpm`. Only applicable when `cmd=start`. 
* `rate` - Rate in integer. Only applicable when `cmd=start`. 
* `imif` - (*optional*) parameter indicating time interval between issued messages. Possible values:
    * `poisson` - emit messages modeled with Poisson point process, whose time intervals are exponential variables with mean 1/rate
    * `uniform` - issues messages at constant rate

Example requests:

```shell
http://localhost:8080/spammer?cmd=start&rate=10&unit=mps

http://localhost:8080/spammer?cmd=start&rate=10&unit=mps&imif=uniform
http://localhost:8080/spammer?cmd=stop
```

## Tangle Width

When running GoShimmer locally in a Docker network, the network delay is so small that only 1 tip will be available most of the time. 
In order to artificially create a tangle structure with multiple tips you can add a `messageLayer.tangleWidth` property to [config.docker.json](https://github.com/iotaledger/goshimmer/blob/develop/tools/docker-network/config.docker.json)
that specifies the number of tips that nodes should retain. This setting exists only for local testing purposes and should not be used in a distributed testnet.  

Here is an example config that can be added: 

```json
  {
  "messageLayer": {
    "tangleWidth": 10
  }}
```

## Running With `docker-compose` Directly

To get an instance up and running on your machine make sure you have [Docker Compose](https://docs.docker.com/compose/install/) installed.

Then you just need to run this command:

```shell
docker-compose build
docker-compose --profile drng --profile grafana up -d
```

> Note: Docker will build the GoShimmer image which can take several minutes.

### Base Components

These services that are created by default with `docker-compose up -d`.

#### Configuration

- SNAPSHOT_FILE: The full path to the message snapshot file. Defaults to `./goshimmer/assets/7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih.bin`
- GOSHIMMER_TAG: (Optional) The [iotaledger/goshimmer](https://hub.docker.com/r/iotaledger/goshimmer) tag to use. Defaults to `develop`.
- GOSHIMMER_CONFIG: The location of the GoShimmer config file. Defaults to `./config.docker.json`.

#### Example

You can set the environment variable configuration inline as seen in this example.

```shell
GOSHIMMER_TAG=develop docker-compose up -d
```

#### Peer master

A node that is used to expose ports via the host and to have a single attachment point for monitoring tools.

##### Volumes

Docker Compose creates a `mainnetdb` volume to maintain a tangle even after tearing down the containers. Run `docker-compose down -v` to clear the volume.

##### Ports

The following ports are exposed on the host to allow for interacting with the Tangle.

| Port | Service |
|------|---------|
| 8080/tcp | Web API | 
| 9000/tcp | Analysis dashboard | 

#### Peer replicas

A node that can be replicated to add more nodes to your local tangle.

##### Ports

These expose 0 ports because they are replicas and the host system cannot map a port to multiple containers.

#### Faucet

A node that can dispense tokens.

##### Ports

The following ports are exposed on the host to allow for interacting with the Tangle.

| Port | Service |
|------|---------|
| 8081/tcp | Dashboard | 
<!-- The dashboard has issues displaying on the master peer when the 2.0 DevNet dashboard is running so we display the dashboard on the faucet -->

### Optional Components

These services can be added to your deployment through `--profile` flags and can be configured with `ENVIRONMENT_VARIABLES`.

#### Grafana + Prometheus

A set of containers to enable dashboards and monitoring.

##### Profile

In order to enable these containers you must set the `--profile grafana` flag when running `docker-compose`.

##### Configuration

- PROMETHEUS_CONFIG: Location of the prometheus config yaml file. Defaults to `./prometheus.yml`.

##### Example

You can set the environment variable configuration inline as seen in this example.

```shell
docker-compose --profile grafana up -d
```

##### Ports

The following ports are exposed on the host to allow for interacting with the Tangle.

| Port | Service |
|------|---------|
| 3000/tcp | Grafana | 
| 9090/tcp | Prometheus | 

#### DRNG

Distributed randomness beacon.
Verifiable, unpredictable and unbiased random numbers as a service.

##### Profile

In order to enable these containers you must set the `--profile drng` flag when running `docker-compose`.

##### Configuration

- DRNG_REPLICAS: (Optional) How many nodes to create in addition to the DRNG leader. Defaults to `2`.

##### Example

You can set the environment variable configuration inline as seen in this example.

```shell
DRNG_REPLICAS=2 docker-compose --profile drng up -d
```

##### Ports

The following ports are exposed on the host to allow for interacting with the Tangle.

| Port | Service |
|------|---------|
| 8000/tcp | Drand Control | 
| 8800/tcp | GoShimmer API | 