#!/bin/bash

SWARM_MANAGER="65.21.49.191"
SWARM_MANAGER_USER="deployer"
export SWARM_ENDPOINT="ssh://$SWARM_MANAGER_USER@$SWARM_MANAGER:22"
export SWARM_REGISTRY="127.0.0.1:5000"

ssh -f -n -N -o ConnectTimeout=3 $SWARM_MANAGER_USER@$SWARM_MANAGER -L $SWARM_REGISTRY:$SWARM_REGISTRY

if [ $? != 0 ];
  echo "Unable to connect to Swarm Manager"
  exit -1
fi

TEST_NAMES='tipsel'

export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1

GOSHIMMER_BASE=$PWD/../..

IMAGE_NAME=$SWARM_REGISTRY/disintegration-$name-$quilt
pushd $GOSHIMMER_BASE
docker built -t $VANILLA_IMAGE_NAME .
docker push $VANILLA_IMAGE_NAME
popd

echo "Run DISintegration tests"

for name in $TEST_NAMES; do
  ( 
    cd tester/tests/$name
    for quilt in quilt_*; do
      BUILD_DIR=$(mktemp -d)
      cp -a $GOSHIMMER_BASE $BUILD_DIR/
      pushd $BUILD_DIR/*
      for patch in tools/disintegration-tests/tester/tests/$name/$quilt/*.patch; do
        patch -p1 <$patch
      done
      IMAGE_NAME=$SWARM_REGISTRY/disintegration-$name-$quilt
      docker build -t $IMAGE_NAME .
      docker push $IMAGE_NAME
      popd
      rm -rf $BUILD_DIR
    done
  )
  pushd $GOSHIMMER_BASE
  TESTER_IMAGE_NAME=$SWARM_REGISTRY/disintegration-$name-tester
  TEST_NAME=$name docker build -t $TESTER_IMAGE_NAME -f tools/disintegration-tests/tester/Dockerfile .
  docker push $TESTER_IMAGE_NAME
  popd
  TESTER_SERVICE=$(docker service create -d $TESTER_IMAGE_NAME)
  docker service logs $TESTER_SERVICE &>logs/"$name"_tester.log
done
