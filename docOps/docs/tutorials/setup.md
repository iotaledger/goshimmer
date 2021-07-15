# Setting up a GoShimmer node 

This page describes how to setup your own GoShimmer node in the GoShimmer testnet with Docker.

> DISCLAIMER: **Note that there will be breaking changes frequently (approx. bi-weekly) where the entire network needs to upgrade. If you don't have time to continuously monitor and upgrade your node, then running a GoShimmer node might not be for you.**  We want to emphasize that running a GoShimmer node requires proper knowledge in Linux and IT related topics such as networking and so on. It is not meant as a node to be run by people with little experience in the mentioned fields. **Do not plan to run any production level services on your node/network.**

| Contents                                                                        |
|:------------------------------------------------------------------------------- |
| [Why you should run a node](#why-you-should-run-a-node)                           |
| [Installing GoShimmer with Docker](#installing-goshimmer-with-docker)           |
| [Running the GoShimmer node](#running-the-goshimmer-node)                       |
| [Managing the GoShimmer node lifecycle](#managing-the-goshimmer-node-lifecycle) |
| [Setting up the Grafana dashboard](#setting-up-the-grafana-dashboard)           |

## Why you should run a node

Running a node in the GoShimmer testnet helps us in the following ways:
* It increases the amount of nodes in the network and thus lets it form a more realistic network.
* Your node will be configured to send debug log messages to a centralized logger from which we can assess and debug research questions and occurring problems.
* Your node is configured to send metric data to a centralized analysis server where we store information such as resource consumption, traffic, FPC vote context processing and so on. This data helps us further fostering the development of GoShimmer and assessing network behavior.
* If you expose your HTTP API port, you provide an entrypoint for other people to interact with the network.

> Note that any metric data is anonymous.

## Installing GoShimmer with Docker

#### Hardware Requirements

> Note that we do not provide a Docker image or binaries for ARM based systems such as Raspberry Pis.

We recommend running GoShimmer on a x86 VPS with following minimum hardware specs:
* 2 cores / 4 threads
* 4 GB of memory
* 40 GB of disk space

A cheap [CX21 Hetzner instance](https://www.hetzner.de/cloud) is thereby sufficient.

If you plan on running your GoShimmer node from home, please only do so if you know how to properly configure NAT on your router, as otherwise your node will not correctly participate in the network.

---

> In the following sections we are going to use a CX21 Hetzner instance with Ubuntu 20.04 while being logged in as root

Lets first upgrade the packages on our system:
```
apt update && apt dist-upgrade -y
```

#### Install Docker

Install needed dependencies:
```
apt-get install \
     apt-transport-https \
     ca-certificates \
     curl \
     gnupg-agent \
     software-properties-common
```

Add Docker’s official GPG key:
```
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
```

Verify that the GPG key matches:
```
apt-key fingerprint 0EBFCD88
pub   rsa4096 2017-02-22 [SCEA]
      9DC8 5822 9FC7 DD38 854A  E2D8 8D81 803C 0EBF CD88
uid           [ unknown] Docker Release (CE deb) <docker@docker.com>
sub   rsa4096 2017-02-22 [S]

```

Add the actual repository:
```
add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"
```

Update the package index:
```
apt-get update
```

And finally, install docker:
```
apt-get install docker-ce docker-ce-cli containerd.io
```

On windows-subsystem for Linux (WSL2) it may be necessary to start docker seperately:
```
/etc/init.d/docker start
```
Note, this may not work on WSL1.

Check whether docker is running by executing `docker ps`:
```
docker ps
CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS               NAMES
```

### Install Docker Compose
Docker compose gives us the ability to define our services with `docker-compose.yml` files instead of having to define all container parameters directly on the CLI.

Download docker compose:
```
curl -L "https://github.com/docker/compose/releases/download/1.26.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
```

Make it executable:
```
chmod +x /usr/local/bin/docker-compose
```

Check that docker compose works:
```
docker-compose --version
docker-compose version 1.26.0, build d4451659
```

### Define the docker-compose.yml

First, lets create a user defined bridged network. Unlike the already existing `bridge` network, the user defined one will have container name DNS resolution for containers within that network. This is useful if later we want to setup additional containers which need to speak with the GoShimmer container.

```
docker network create --driver=bridge shimmer
c726034d295c3df66803b92c71ca517a0cf0e3c65c1c6d84ee5fa34ae76cbcd4
```

Lets create a folder holding our `docker-compose.yml`:
```
mkdir /opt/goshimmer
```

Lets create a folder holding our database:
```
cd /opt/goshimmer
mkdir db
chmod 0777 db
```

Finally, lets create our `docker-compose.yml`:
```
nano docker-compose.yml
```

and add following content:
```yaml
version: '3.3'

networks:
  outside:
    external:
      name: shimmer

services:
  goshimmer:
    image: iotaledger/goshimmer:latest
    container_name: goshimmer
    hostname: goshimmer
    stop_grace_period: 2m
    volumes:
      - "./db:/tmp/mainnetdb:rw"   
      - "/etc/localtime:/etc/localtime:ro"
    ports:
      # Autopeering 
      - "0.0.0.0:14626:14626/udp"
      # Gossip
      - "0.0.0.0:14666:14666/tcp"
      # FPC
      - "0.0.0.0:10895:10895/tcp"
      # HTTP API
      - "0.0.0.0:8080:8080/tcp"
      # Dashboard
      - "0.0.0.0:8081:8081/tcp"
      # pprof profiling
      - "0.0.0.0:6061:6061/tcp"
    environment:
      - ANALYSIS_CLIENT_SERVERADDRESS=ressims.iota.cafe:21888
      - AUTOPEERING_PORT=14626
      - DASHBOARD_BINDADDRESS=0.0.0.0:8081
      - GOSSIP_PORT=14666
      - WEBAPI_BINDADDRESS=0.0.0.0:8080
      - PROFILING_BINDADDRESS=0.0.0.0:6061
      - NETWORKDELAY_ORIGINPUBLICKEY=9DB3j9cWYSuEEtkvanrzqkzCQMdH1FGv3TawJdVbDxkd
      - FPC_BINDADDRESS=0.0.0.0:10895
      - PROMETHEUS_BINDADDRESS=0.0.0.0:9311
    command: >
      --skip-config=true
      --autopeering.entryNodes=2PV5487xMw5rasGBXXWeqSi4hLz7r19YBt8Y1TGAsQbj@ressims.iota.cafe:15626,5EDH4uY78EA6wrBkHHAVBWBMDt7EcksRq6pjzipoW15B@entry-devnet.tanglebay.com:14646
      --node.disablePlugins=
      --node.enablePlugins=remotelog,networkdelay,spammer,prometheus
      --logger.level=info
      --logger.disableEvents=false
      --logger.remotelog.serverAddress=ressims.iota.cafe:5213
      --drng.pollen.instanceId=1
      --drng.pollen.threshold=3
      --drng.pollen.committeeMembers=AheLpbhRs1XZsRF8t8VBwuyQh9mqPHXQvthV5rsHytDG,FZ28bSTidszUBn8TTCAT9X1nVMwFNnoYBmZ1xfafez2z,GT3UxryW4rA9RN9ojnMGmZgE2wP7psagQxgVdA4B9L1P,4pB5boPvvk2o5MbMySDhqsmC2CtUdXyotPPEpb7YQPD7,64wCsTZpmKjRVHtBKXiFojw7uw3GszumfvC4kHdWsHga
      --drng.xteam.instanceId=1339
      --drng.xteam.threshold=4
      --drng.xteam.committeeMembers=GUdTwLDb6t6vZ7X5XzEnjFNDEVPteU7tVQ9nzKLfPjdo,68vNzBFE9HpmWLb2x4599AUUQNuimuhwn3XahTZZYUHt,Dc9n3JxYecaX3gpxVnWb4jS3KVz1K1SgSK1KpV1dzqT1,75g6r4tqGZhrgpDYZyZxVje1Qo54ezFYkCw94ELTLhPs,CN1XLXLHT9hv7fy3qNhpgNMD6uoHFkHtaNNKyNVCKybf,7SmttyqrKMkLo5NPYaiFoHs8LE6s7oCoWCQaZhui8m16,CypSmrHpTe3WQmCw54KP91F5gTmrQEL7EmTX38YStFXx
    networks:
      - outside
```

> If performance is a concern, you can also run your containers with `network_mode: "host"`, however, you must then adjust the hostnames in the configs for the corresponding containers and perhaps also create some iptable rules to block traffic from outside accessing your services directly.

Note how we are setting up NATs for different ports:


| Port  | Functionality  | Protocol |
| ----- | -------------- | -------- |
| 14626 | Autopeering    | UDP      |
| 14666 | Gossip         | TCP      |
| 10895 | FPC            | TCP/HTTP |
| 8080  | HTTP API      | TCP/HTTP |
| 8081  | Dashboard       | TCP/HTTP |
| 6061  | pprof HTTP API | TCP/HTTP |

It is important that the ports are correctly mapped so that the node for example actively participates in FPC votes or can gain inbound neighbors.

> If the UDP NAT mapping is not configured correctly, GoShimmer will terminate with an error message stating to check the NAT configuration

## Running the GoShimmer node

Within the `/opt/goshimmer` folder where the `docker-compose.yml` resides, simply execute:
```
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
```
docker logs -f --since=1m goshimmer
```

### Syncing

When the node starts for the first time, it must synchronize its state with the rest of the network. GoShimmer currently uses the Tangle Time to help nodes determine their synced status.

#### Dashboard
The dashboard of your GoShimmer node should be accessible via `http://<your-ip>:8081`. If your node is still synchronizing, you might see a higher inflow of MPS.

![](https://user-images.githubusercontent.com/11289354/119599542-c3985e00-be17-11eb-8769-7e639f365ae5.png)

After a while, your node's dashboard should also display up to 8 neighbors:
![](https://i.imgur.com/gAyAXK9.png)


#### HTTP API
GoShimmer also exposes an HTTP API. To check whether that works correctly, you can access it via `http://<your-ip>:8080/info` which should return a JSON response in the form of:
```
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

##### Stopping the node
```
docker-compose stop
```

##### Resetting the node
```
docker-compose down
```

##### Upgrading the node
**Ensure that the image version in the `docker-compose.yml` is `latest`** then execute following commands:
```
docker-compose down
rm db/*
docker-compose pull
docker-compose up -d
```

##### Following log output
```
docker logs -f --since=1m goshimmer
```

##### Create a log.txt
```
docker logs goshimmer > log.txt
```
##### Update Grafana Dashboard
If you set up the Grafana dashboard for your node according to the next section "Setting up the Grafana dashboard", the following method will help you to update when a new version is released.

You have to manually copy the new [dashboard file](https://github.com/iotaledger/goshimmer/blob/master/tools/monitoring/grafana/dashboards/local_dashboard.json) into `/opt/goshimmer/grafana/dashboards` directory.
Supposing you are at `/opt/goshimmer/`:
```
wget https://raw.githubusercontent.com/iotaledger/goshimmer/master/tools/monitoring/grafana/dashboards/local_dashboard.json
cp local_dashboard.json grafana/dashboards
```
Restart the grafana container:
```
docker restart grafana
```


## Setting up the Grafana dashboard

#### Add Prometheus and Grafana Containers to `docker-compose.yml`
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
```
cd /opt/goshimmer
mkdir -p prometheus/data
```
2. Create a `prometheus.yml` in `prometheus` directory:
```
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
```
chmod -R 777 prometheus
```
#### Create Grafana configs
1. Create necessary config dirs in `/opt/goshimmer/`.
```
mkdir -p grafana/provisioning/datasources grafana/provisioning/dashboards grafana/provisioning/notifiers grafana/provisioning/plugins
mkdir -p grafana/dashboards
```
2. Create a datasource configuration file in `grafana/provisioning/datasources`:
```
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
```
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

Head over to the GoShimmer repository and download [local_dashboard.json](https://github.com/iotaledger/goshimmer/blob/master/tools/monitoring/grafana/dashboards/local_dashboard.json).
```
wget https://raw.githubusercontent.com/iotaledger/goshimmer/master/tools/monitoring/grafana/dashboards/local_dashboard.json
cp local_dashboard.json grafana/dashboards
```
5. Add permissions to Grafana config folder
```
chmod -R 777 grafana
```
#### Run GoShimmer with Prometheus and Grafana:
```
docker-compose up -d
```

The Grafana dashboard should be accessible at `http://<your-ip>:3000`.

Default login credentials are:
* `username`: admin
* `password`: admin
