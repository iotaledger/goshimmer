#!/bin/bash

wget http://localhost:8080/snapshot -O TEST_snapshot.bin

docker run -it -v $PWD/TEST_snapshot.bin:/app/snapshot.bin --network docker-network_goshimmer --name joiner \
  -p 8011:8081 \
  docker-network-peer_master:latest \
  --config=/app/config.json --autoPeering.bindAddress=0.0.0.0:14626 --autoPeering.entryNodes=EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP@peer_master:14626 --blockIssuer.rateSetter.mode=disabled --dashboard.bindAddress=0.0.0.0:8081 --dashboard.dev=false --database.directory=/app/db --dagsvisualizer.dev=false --dagsvisualizer.devBindAddress=dagsvisualizer-dev-docker:3000 --logger.disableCaller=false --logger.disableEvents=true --logger.disableStacktrace=false --logger.encoding=console --logger.level=info --logger.outputPaths=stdout --metrics.bindAddress=0.0.0.0:9311 --metrics.goMetrics=true --metrics.processMetrics=true --node.overwriteStoredSeed=true --node.peerDBDirectory=/app/peerdb --profiling.bindAddress=0.0.0.0:6061 --protocol.snapshot.path=./snapshot.bin --remotemetrics.metricsLevel=0 --webAPI.basicAuth.enabled=false --webAPI.basicAuth.password=goshimmer --webAPI.basicAuth.username=goshimmer --webAPI.bindAddress=0.0.0.0:8080 --node.enablePlugins=metrics,spammer,WebAPIToolsBlockEndpoint --node.disablePlugins=portcheck,ManaInitializer,RemoteLog
