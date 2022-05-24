---
description: Create a static GoShimmer identity using a random 32byte autopeering seed, open ssl or rand-seed. 
image: /img/logo/goshimmer_light.png
keywords:
- seed
- static
- identity
- tools
- base64
---

# Create a Static Identity

To create a static GoShimmer identity, you will need to generate a random 32 byte seed. You can use `openssl` or the `rand-seed` tool we provide under the GoShimmer folder `tools/rand-seed`.
For example, by running:
* `openssl rand -base64 32`: generates a random 32 byte sequence encoded in base64. The output should look like: `gP0uRLhwBG2yJJmnLySX4S4R5G250Z3dbN9yBR6VSyY=`
* `go run main.go` under the GoShimmer folder `tools/rand-seed`: generates a random 32 byte sequence encoded in both base64 and base58. The output is written into the file `random-seed.txt` and should look like:
```
base64:nQW9MhNSLpIqBUiZe90XI320g680zxFoB1UIK09Acus=
base58:BZx5tDLymckUV5wiswXJtajgQrBEzTBBRR4uGfr1YNGS
```

You can now copy one of that strings (together with the encoding type prefix) and paste it into the GoShimmer `config.json` under the `node` section:

```json
"node": {
  "seed":"base64:gP0uRLhwBG2yJJmnLySX4S4R5G250Z3dbN9yBR6VSyY=",
  "disablePlugins": [],
  "enablePlugins": []
},
``` 

Or if you are using docker and prefer to set this with a command, you can define the same by changing the GoShimmer docker-compose.yml:
```yaml
goshimmer:
  network_mode: host
  image: iotaledger/goshimmer
  build:
    context: ./
    dockerfile: Dockerfile
  container_name: iota_goshimmer
  command: >
    --node.enablePlugins=prometheus
    --node.seed="base64:gP0uRLhwBG2yJJmnLySX4S4R5G250Z3dbN9yBR6VSyY="
    # Mount volumes:
    # make sure to give read/write access to the folder ./mainnetdb (e.g., chmod -R 777 ./mainnetdb)
    # optionally, you can mount a config.json into the container
    volumes:
      - ./mainnetdb/:/app/mainnetdb/:rw
      - ./config.json:/app/config.json:ro
    # Expose ports:
    # gossip:       - "14666:14666/tcp"
    # autoPeering:  - "14626:14626/udp"
    # webAPI:       - "8080:8080/tcp"
    # dashboard:    - "8081:8081/tcp"
    ports:
      - "14666:14666/tcp"
      - "14626:14626/udp"
      - "9311:9311/tcp" # prometheus exporter
      - "8080:8080/tcp" # webApi
      - "8081:8081/tcp" # dashboard
```
