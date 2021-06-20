#!/bin/bash
echo "running tests..."
go test ./tests/"${TEST_NAME}" -v -timeout 30m
