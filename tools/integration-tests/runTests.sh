#!/bin/bash

DEFAULT_TEST_NAMES='autopeering common consensus drng value faucet mana diagnostics'
TEST_NAMES=${1:-$DEFAULT_TEST_NAMES}

export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1
echo "Build GoShimmer image"
docker build --build-arg REMOTE_DEBUGGING=1 --build-arg DOWNLOAD_SNAPSHOT=0 -t iotaledger/goshimmer ../../.

echo "Pull additional Docker images"
docker pull angelocapossele/drand:v1.1.4
docker pull gaiaadm/pumba:0.7.2
docker pull gaiadocker/iproute2:latest
docker pull alpine/socat:1.7.4.3-r0

echo "Run integration tests"

for name in $TEST_NAMES; do
  TEST_NAME=$name docker-compose -f tester/docker-compose.yml up --abort-on-container-exit --exit-code-from tester --build
  docker logs tester &>logs/"$name"_tester.log
done

echo "Clean up"
docker-compose -f tester/docker-compose.yml down
docker rm -f $(docker ps -a -q -f ancestor=gaiadocker/iproute2)
