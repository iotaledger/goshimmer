#!/bin/bash

echo "Build GoShimmer image"
docker build -t iotaledger/goshimmer ../../.

echo "Run integration tests"
docker-compose -f tester/docker-compose.yml up --abort-on-container-exit --exit-code-from tester --build

echo "Create logs from containers in network"
docker logs tester &> logs/tester.log

echo "Clean up"
docker-compose -f tester/docker-compose.yml down