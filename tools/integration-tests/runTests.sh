#!/bin/bash

TEST_NAMES='autopeering common drng message value consensus faucet syncbeacon'

echo "Build GoShimmer image"
docker build -t iotaledger/goshimmer ../../.

echo "Pull additional Docker images"
docker pull angelocapossele/drand:v1.1.4
docker pull gaiaadm/pumba:0.7.2
docker pull gaiadocker/iproute2:latest

echo "Run integration tests"

for name in $TEST_NAMES
do
  TEST_NAME=$name docker-compose -f tester/docker-compose.yml up --abort-on-container-exit --exit-code-from tester --build
  docker logs tester &> logs/"$name"_tester.log
done

echo "Clean up"
docker-compose -f tester/docker-compose.yml down
docker rm -f $(docker ps -a -q -f ancestor=gaiadocker/iproute2)
