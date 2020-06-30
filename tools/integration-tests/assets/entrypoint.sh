#!/bin/bash
echo "copying assets into shared volume..."
rm -rf /assets/*
cp -rp /tmp/assets/* /assets
chmod 777 /assets/*
echo "assets:"
ls /assets
echo "running tests..."
go test ./tests/"${TEST_NAME}" -v -timeout 30m
