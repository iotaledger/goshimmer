#!/bin/bash

# Create a function to join an array by a character
function join { local IFS="$1"; shift; echo "$*"; }

if [[ $# -gt 4 ]] ; then
    echo 'Call with ./run [replicas=1|2|3|...] [grafana=0|1] [drng=0|1]'
    exit 0
fi

REPLICAS=${1:-1}
GRAFANA=${2:-0}
DRNG=${3:-0}

export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1
echo "Build GoShimmer"
docker-compose build

# check exit code of builder
if [ $? -ne 0 ]
then
  echo "Building failed. Please fix and try again!"
  exit 1
fi

echo "Run GoShimmer network"
export SHIMMER_PEER_REPLICAS=$REPLICAS
PROFILES=()
if [ $GRAFANA -ne 0 ]
then
  export MONGO_DB_ENABLED=true
  PROFILES+=("grafana")
fi

if [ $DRNG -ne 0 ]
then
  PROFILES+=("drng")
fi
export COMPOSE_PROFILES=$(join , ${PROFILES[@]})
docker-compose up

echo "Clean up docker resources"
docker-compose down -v