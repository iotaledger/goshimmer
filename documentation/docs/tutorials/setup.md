---
description:  How to set up your own GoShimmer node in the GoShimmer testnet with Docker
image: /img/tutorials/setup/dashboard.png
keywords:
- node
- set up
- docker
- http API
- tcp
- dashboard
- prometheus
- grafana
---
# Setting up a GoShimmer node

This page describes how to set up your own GoShimmer node in the GoShimmer testnet with Docker.

:::warning DISCLAIMER
**Note that there will be breaking changes frequently (approx. bi-weekly) where the entire network needs to upgrade. If you don't have time to continuously monitor and upgrade your node, then running a GoShimmer node might not be for you.**

We want to emphasize that running a GoShimmer node requires proper knowledge in Linux and IT related topics such as networking and so on. It is not meant as a node to be run by people with little experience in the mentioned fields. **Do not plan to run any production level services on your node/network.**
:::


## Why You Should Run a Node

Running a node in the GoShimmer testnet helps us in the following ways:
* It increases the amount of nodes in the network and thus lets it form a more realistic network.
* Your node will be configured to send debug log messages to a centralized logger from which we can assess and debug research questions and occurring problems.
* Your node is configured to send metric data to a centralized analysis server where we store information such as resource consumption, traffic, and so on. This data helps us further fostering the development of GoShimmer and assessing network behavior.
* If you expose your HTTP API port, you provide an entrypoint for other people to interact with the network.

:::note

Any metric data is anonymous.

:::

## Installing GoShimmer with Docker

#### Hardware Requirements

:::note

We do not provide a Docker image or binaries for ARM based systems such as Raspberry Pis.

:::

We recommend running GoShimmer on a x86 VPS with following minimum hardware specs:
* 2 cores / 4 threads
* 4 GB of memory
* 40 GB of disk space

A cheap [CX21 Hetzner instance](https://www.hetzner.de/cloud) is thereby sufficient.

If you plan on running your GoShimmer node from home, please only do so if you know how to properly configure NAT on your router, as otherwise your node will not correctly participate in the network.

---

:::info

In the following sections we are going to use a CX21 Hetzner instance with Ubuntu 20.04 while being logged in as root

:::

Lets first upgrade the packages on our system:
```shell
apt update && apt dist-upgrade -y
```

#### Install Docker

Install needed dependencies:

```shell
apt-get install \
     apt-transport-https \
     ca-certificates \
     curl \
     gnupg-agent \
     software-properties-common
```

Add Docker’s official GPG key:

```shell
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
```

Verify that the GPG key matches:

```shell
apt-key fingerprint 0EBFCD88
pub   rsa4096 2017-02-22 [SCEA]
      9DC8 5822 9FC7 DD38 854A  E2D8 8D81 803C 0EBF CD88
uid           [ unknown] Docker Release (CE deb) <docker@docker.com>
sub   rsa4096 2017-02-22 [S]

```

Add the actual repository:

```shell
add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"
```

Update the package index:

```shell
apt-get update
```

And finally, install docker:

```shell
apt-get install docker-ce docker-ce-cli containerd.io
```

On windows-subsystem for Linux (WSL2) it may be necessary to start docker seperately:
```
/etc/init.d/docker start
```
Note, this may not work on WSL1.

Check whether docker is running by executing `docker ps`:

```shell
docker ps
CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS               NAMES
```

### Install Docker Compose

Docker compose gives us the ability to define our services with `docker-compose.yml` files instead of having to define all container parameters directly on the CLI.

Download docker compose:

```shell
curl -L "https://github.com/docker/compose/releases/download/1.26.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
```

Make it executable:

```shell
chmod +x /usr/local/bin/docker-compose
```

Check that docker compose works:

```shell
docker-compose --version
docker-compose version 1.26.0, build d4451659
```

### Define the docker-compose.yml

First, lets create a user defined bridged network. Unlike the already existing `bridge` network, the user defined one will have container name DNS resolution for containers within that network. This is useful if later we want to setup additional containers which need to speak with the GoShimmer container.

```shell
docker network create --driver=bridge goshimmer
c726034d295c3df66803b92c71ca517a0cf0e3c65c1c6d84ee5fa34ae76cbcd4
```

Lets create a folder holding our `docker-compose.yml`:

```shell
mkdir /opt/goshimmer
```

Lets create a folder holding our database:

```shell
cd /opt/goshimmer
sudo mkdir mainnetdb && sudo chown 65532:65532 mainnetdb
sudo mkdir peerdb && sudo chown 65532:65532 peerdb
```

Finally, lets create our `docker-compose.yml`:

```shell
nano docker-compose.yml
```

and add following content:
```yaml
version: '3.3'

networks:
  outside:
    external:
      name: goshimmer

services:
  goshimmer:
    image: iotaledger/goshimmer:latest
    container_name: goshimmer
    hostname: goshimmer
    stop_grace_period: 2m
    volumes:
      - "./db:/app/mainnetdb:rw"
      - "./peerdb:/app/peerdb:rw"
      - "/etc/localtime:/etc/localtime:ro"
    ports:
      # Autopeering
      - "0.0.0.0:14626:14626/udp"
      # Gossip
      - "0.0.0.0:14666:14666/tcp"
      # HTTP API
      - "0.0.0.0:8080:8080/tcp"
      # Dashboard
      - "0.0.0.0:8081:8081/tcp"
      # pprof profiling
      - "0.0.0.0:6061:6061/tcp"
    environment:
      - ANALYSIS_CLIENT_SERVERADDRESS=analysisentry-01.devnet.shimmer.iota.cafe:21888
      - AUTOPEERING_BINDADDRESS=0.0.0.0:14626
      - DASHBOARD_BINDADDRESS=0.0.0.0:8081
      - GOSSIP_BINDADDRESS=0.0.0.0:14666
      - WEBAPI_BINDADDRESS=0.0.0.0:8080
      - PROFILING_BINDADDRESS=0.0.0.0:6061
      - NETWORKDELAY_ORIGINPUBLICKEY=9DB3j9cWYSuEEtkvanrzqkzCQMdH1FGv3TawJdVbDxkd
      - PROMETHEUS_BINDADDRESS=0.0.0.0:9311
    command: >
      --skip-config=true
      --autoPeering.entryNodes=2PV5487xMw5rasGBXXWeqSi4hLz7r19YBt8Y1TGAsQbj@analysisentry-01.devnet.shimmer.iota.cafe:15626,5EDH4uY78EA6wrBkHHAVBWBMDt7EcksRq6pjzipoW15B@entry-0.devnet.tanglebay.com:14646,CAB87iQZR6BjBrCgEBupQJ4gpEBgvGKKv3uuGVRBKb4n@entry-1.devnet.tanglebay.com:14646
      --node.disablePlugins=portcheck
      --node.enablePlugins=remotelog,networkdelay,spammer,prometheus
      --database.directory=/app/mainnetdb
      --node.peerDBDirectory=/app/peerdb
      --logger.level=info
      --logger.disableEvents=false
      --logger.remotelog.serverAddress=metrics-01.devnet.shimmer.iota.cafe:5213
      --drng.pollen.instanceID=1
      --drng.pollen.threshold=3
      --drng.pollen.committeeMembers=AheLpbhRs1XZsRF8t8VBwuyQh9mqPHXQvthV5rsHytDG,FZ28bSTidszUBn8TTCAT9X1nVMwFNnoYBmZ1xfafez2z,GT3UxryW4rA9RN9ojnMGmZgE2wP7psagQxgVdA4B9L1P,4pB5boPvvk2o5MbMySDhqsmC2CtUdXyotPPEpb7YQPD7,64wCsTZpmKjRVHtBKXiFojw7uw3GszumfvC4kHdWsHga
      --drng.xTeam.instanceID=1339
      --drng.xTeam.threshold=4
      --drng.xTeam.committeeMembers=GUdTwLDb6t6vZ7X5XzEnjFNDEVPteU7tVQ9nzKLfPjdo,68vNzBFE9HpmWLb2x4599AUUQNuimuhwn3XahTZZYUHt,Dc9n3JxYecaX3gpxVnWb4jS3KVz1K1SgSK1KpV1dzqT1,75g6r4tqGZhrgpDYZyZxVje1Qo54ezFYkCw94ELTLhPs,CN1XLXLHT9hv7fy3qNhpgNMD6uoHFkHtaNNKyNVCKybf,7SmttyqrKMkLo5NPYaiFoHs8LE6s7oCoWCQaZhui8m16,CypSmrHpTe3WQmCw54KP91F5gTmrQEL7EmTX38YStFXx
    networks:
      - outside
```

:::info

If performance is a concern, you can also run your containers with `network_mode: "host"`, however, you must then adjust the hostnames in the configs for the corresponding containers and perhaps also create some iptable rules to block traffic from outside accessing your services directly.

:::warning INFO

If your home network is IPv6-only (as is common for some ISPs in a few countries like Germany), make sure your docker installation is configured to support IPv6 as this is not always the default setting. If your ports and firewalls are configured correctly and your GoShimmer node does start but does not seem to find any neighbors even after a little while, this might be the solution to your problem. Find the very short guide to enable IPv6 support for docker in the [Docker documentation](https://docs.docker.com/config/daemon/ipv6/).

:::

Note how we are setting up NATs for different ports:


| Port  | Functionality  | Protocol |
| ----- | -------------- | -------- |
| 14626 | Autopeering    | UDP      |
| 14666 | Gossip         | TCP      |
| 8080  | HTTP API      | TCP/HTTP |
| 8081  | Dashboard       | TCP/HTTP |
| 6061  | pprof HTTP API | TCP/HTTP |

It is important that the ports are correctly mapped so that the node can gain inbound neighbors.

:::warning INFO

If the UDP NAT mapping is not configured correctly, GoShimmer will terminate with an error message stating to check the NAT configuration

:::

## Running the GoShimmer Node

Within the `/opt/goshimmer` folder where the `docker-compose.yml` resides, simply execute:

```shell
docker-compose up -d
Pulling goshimmer (iotaledger/goshimmer:0.2.0)...
...
```
to start the GoShimmer node.

You should see your container running now:
```
CONTAINER ID        IMAGE               COMMAND                  CREATED             STATUS              PORTS                                                                                                                                    NAMES
687f52b78cb5        iotaledger/goshimmer:0.2.0       "/run/goshimmer --sk…"   19 seconds ago      Up 17 seconds       0.0.0.0:6061->6061/tcp, 0.0.0.0:8080-8081->8080-8081/tcp, 0.0.0.0:10895->10895/tcp, 0.0.0.0:14666->14666/tcp, 0.0.0.0:14626->14626/udp   goshimmer
```

You can follow the log output of the node via:

```shell
docker logs -f --since=1m goshimmer
```

### Syncing

When the node starts for the first time, it must synchronize its state with the rest of the network. GoShimmer currently uses the Tangle Time to help nodes determine their synced status.

#### Dashboard
The dashboard of your GoShimmer node should be accessible via `http://<your-ip>:8081`. If your node is still synchronizing, you might see a higher inflow of MPS.

[![GoShimmer Dashboard](/img/tutorials/setup/dashboard.png)](/img/tutorials/setup/dashboard.png)

After a while, your node's dashboard should also display up to 8 neighbors:
[![GoShimmer Dashboard Neighbors](/img/tutorials/setup/dashboard_neighbors.png)](/img/tutorials/setup/dashboard_neighbors.png)


#### HTTP API
GoShimmer also exposes an HTTP API. To check whether that works correctly, you can access it via `http://<your-ip>:8080/info` which should return a JSON response in the form of:

```json
{
  "version": "v0.6.2",
  "networkVersion": 30,
  "tangleTime": {
    "messageID": "6ndfmfogpH9H8C9X9Fbb7Jmuf8RJHQgSjsHNPdKUUhoJ",
    "time": 1621879864032595415,
    "synced": true
  },
  "identityID": "D9SPFofAGhA5V9QRDngc1E8qG9bTrnATmpZMdoyRiBoW",
  "identityIDShort": "XBgY5DsUPng",
  "publicKey": "9DB3j9cWYSuEEtkvanrzqkzCQMdH1FGv3TawJdVbDxkd",
  "solidMessageCount": 74088,
  "totalMessageCount": 74088,
  "enabledPlugins": [
    ...
  ],
  "disabledPlugins": [
    ...
  ],
  "mana": {
    "access": 1,
    "accessTimestamp": "2021-05-24T20:11:05.451224937+02:00",
    "consensus": 10439991680906,
    "consensusTimestamp": "2021-05-24T20:11:05.451228137+02:00"
  },
  "manaDelegationAddress": "1HMQic52dz3xLY2aeDXcDhX53LgbsHghdfD8eGXR1qVHy",
  "mana_decay": 0.00003209,
  "scheduler": {
    "running": true,
    "rate": "5ms",
    "nodeQueueSizes": {}
  },
  "rateSetter": {
    "rate": 20000,
    "size": 0
  }
}
```

## Managing the GoShimmer node lifecycle

### Stopping the Node

```shell
docker-compose stop
```

### Resetting the Node

```shell
docker-compose down
```

### Upgrading the Node

**Ensure that the image version in the `docker-compose.yml` is `latest`** then execute following commands:

```shell
docker-compose down
rm db/*
docker-compose pull
docker-compose up -d
```

### Following Log Output

```shell
docker logs -f --since=1m goshimmer
```

### Create a log.txt

```shell
docker logs goshimmer > log.txt
```
### Update Grafana Dashboard

If you set up the Grafana dashboard for your node according to the next section "Setting up the Grafana dashboard", the following method will help you to update when a new version is released.

You have to manually copy the new [dashboard file](https://github.com/iotaledger/goshimmer/blob/develop/tools/docker-network/grafana/dashboards/local_dashboard.json) into `/opt/goshimmer/grafana/dashboards` directory.
Supposing you are at `/opt/goshimmer/`:

```shell
wget https://raw.githubusercontent.com/iotaledger/goshimmer/develop/tools/docker-network/grafana/dashboards/local_dashboard.json
cp local_dashboard.json grafana/dashboards
```
Restart the grafana container:

```shell
docker restart grafana
```


## Setting up the Grafana dashboard

### Add Prometheus and Grafana Containers to `docker-compose.yml`

Append the following to the previously described `docker-compose.yml` file (**make sure to also copy the space in front of "prometheus"/the entire whitespace**):
```yaml
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    restart: unless-stopped
    ports:
      - "9090:9090/tcp"
    command:
      - --config.file=/etc/prometheus/prometheus.yml
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - ./prometheus/data:/prometheus:rw
    depends_on:
      - goshimmer
    networks:
      - outside

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    restart: unless-stopped
    environment:
      # path to provisioning definitions can only be defined as
      # environment variables for grafana within docker
      - GF_PATHS_PROVISIONING=/var/lib/grafana/provisioning
    ports:
      - "3000:3000/tcp"
    user: "472"
    volumes:
      - ./grafana:/var/lib/grafana:rw
    networks:
      - outside
```

#### Create Prometheus config

1. Create a `prometheus/data` directory in `/opt/goshimmer`:

```shell
cd /opt/goshimmer
mkdir -p prometheus/data
```
2. Create a `prometheus.yml` in `prometheus` directory:

```shell
nano prometheus/prometheus.yml
```
The content of the file should be:
```yaml
scrape_configs:
    - job_name: goshimmer_local
      scrape_interval: 5s
      static_configs:
      - targets:
        - goshimmer:9311
```
3. Add permissions to `prometheus` config directory:

```shell
chmod -R 777 prometheus
```

#### Create Grafana configs

1. Create necessary config dirs in `/opt/goshimmer/`.

```shell
mkdir -p grafana/provisioning/datasources grafana/provisioning/dashboards grafana/provisioning/notifiers grafana/provisioning/plugins
mkdir -p grafana/dashboards
```
2. Create a datasource configuration file in `grafana/provisioning/datasources`:

```shell
nano grafana/provisioning/datasources/datasources.yaml
```
With the following content:
```yaml
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    # <string, required> access mode. proxy or direct (Server or Browser in the UI). Required
    access: proxy
    orgId: 1
    url: http://prometheus:9090
    jsonData:
      graphiteVersion: '1.1'
      timeInterval: '1s'
    # <string> json object of data that will be encrypted.
    secureJsonData:
      # <string> database password, if used
      password:
      # <string> basic auth password
      basicAuthPassword:
    version: 1
    # <bool> allow users to edit datasources from the UI.
    editable: true
```
3. Create a dashboard configuration file in `grafana/provisioning/dashboards`:

```shell
nano grafana/provisioning/dashboards/dashboards.yaml
```
With the following content:
```yaml
apiVersion: 1

providers:
  - name: 'GoShimmer Local Metrics'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    editable: true
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /var/lib/grafana/dashboards
```
4. Add predefined GoShimmer Local Metrics Dashboard.

Head over to the GoShimmer repository and download [local_dashboard.json](https://github.com/iotaledger/goshimmer/blob/develop/tools/docker-network/grafana/dashboards/local_dashboard.json).

```shell
wget https://raw.githubusercontent.com/iotaledger/goshimmer/develop/tools/docker-network/grafana/dashboards/local_dashboard.json
cp local_dashboard.json grafana/dashboards
```
5. Add permissions to Grafana config folder

```shell
chmod -R 777 grafana
```

#### Run GoShimmer with Prometheus and Grafana:

```shell
docker-compose up -d
```

The Grafana dashboard should be accessible at `http://<your-ip>:3000`.

Default login credentials are:
* `username`: admin
* `password`: admin

## Installing Goshimmer by Building From Source

### Software Requirements
Upgrade your systems' packages by running the following command:

```shell
apt update && apt dist-upgrade -y
```

#### Installing RocksDB Compression Libraries

GoShimmer uses RocksDB as its underlying database engine. That requires installing its compression libraries. Please use the tutorial from RocksDB's Github:

```shell
https://github.com/facebook/rocksdb/blob/main/INSTALL.md
```

####  GCC and G++

GCC and G++ are required for the compilation to work properly.  You can install them by running the following command:

```shell
sudo apt install gcc g++
```

#### Installing Golang-go

In order for the build script to work later on, we have to install the programming language Go. Which version you need to install is specified in:

```shell
https://github.com/iotaledger/goshimmer/blob/4e3ff2d23d65ddd31053f195fb40d530ef62acf3/go.mod#L3
```

Use apt to install:

```shell
apt install golang-go
```

Check the go version:

```shell
go version
```

If apt did not install the correct go version, use the tutorial provided by the go.dev page:

```shell
https://go.dev/doc/install
```

Use `go version` to check if it successfully installed golang-go.


### Clone the Repository

Once you have installed the [software requirements](#software-requirements), you should clone the [GoShimmer repository](https://github.com/iotaledger/goshimmer/) into the `/opt` directory. You can do so by running the following commands:

```shell
cd /opt
git clone https://github.com/iotaledger/goshimmer.git
```

### Download the Snapshot
You can download the latest snapshot by running the following command from the goshimmer directory you created when you [cloned the repository](#clone-the-repository):

```shell
sudo wget -O snapshot.bin https://dbfiles-goshimmer.s3.eu-central-1.amazonaws.com/snapshots/nectar/snapshot-latest.bin
```

### Making the Node Dashboard Accessible

You will need to modify your goshimmer configuration file to make the Node Dashboard accessible. Below we described a method using the nano text editor, but you can use your text editor of choice.

```shell
nano config.default.json
```

In the config file where it says **dashboard**, change the **bindAddress** from `"127.0.0.1:8081"` to `"0.0.0.0:8081"`.

Rename the file to config.json and save your changes.

:::note
If you do not save the file as `config.json`, the node dashboard will not be accessible through your browser.
:::

### Run the GoShimmer Node

You can now run the build script for the goshimmer binary with the following command:

```shell
./scripts/build.sh
```

:::tip
You can use the `screen` command to keep the node running if you terminate your current ssh session.
:::


You can now run the GoShimmer binary to start your node:

```shell
./goshimmer
```

You can "detach" from the GoShimmer screen by pressing your `CTRL+A+D` keys. This will remove the GoShimmer window,  but it will still be running.

You need the number from the start of the window name to reattach it. If you forget it, you can always use the `-ls` (list) option, as shown below, to get a list of the detached windows:

```shell
screen -ls
```

You can use the -r (reattach) option and the number of the session to reattach it, like so:

```shell
screen -r (your session id)
```

### Stopping the Node

To stop a screen session and your GoShimmer node press `CTRL+A+K` inside the running window. This will stop your screen session.
