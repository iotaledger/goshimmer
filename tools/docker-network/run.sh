#!/bin/bash

if [[ $# -eq 0 ]] ; then
    echo 'Call with ./run replicas [grafana=0|1]'
    exit 0
fi

REPLICAS=$1
GRAFANA=${2:-0}

echo "Build GoShimmer"
docker-compose -f builder/docker-compose.builder.yml up --abort-on-container-exit --exit-code-from builder

# check exit code of builder
if [ $? -ne 0 ]
then
  echo "Building failed. Please fix and try again!"
  exit 1
fi

echo "Run GoShimmer network"
if [ $GRAFANA -ne 0 ]
then
  MONGO_DB_ENABLED=true docker-compose -f docker-compose.yml -f docker-compose-grafana.yml up --scale peer_replica=$REPLICAS
else
  MONGO_DB_ENABLED=false docker-compose -f docker-compose.yml up --scale peer_replica=$REPLICAS
fi

echo "Clean up docker network"
docker-compose down --remove-orphans