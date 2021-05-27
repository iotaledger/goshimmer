#!/bin/bash
#
# Builds both the node's and the analysis dashboard and runs pkger afterwards.

echo "::: Building /plugins/dashboard/frontend :::"
cd ./plugins/dashboard/frontend
yarn install
yarn build

echo "::: Building /plugins/analysis/dashboard/frontend :::"
cd ../../analysis/dashboard/frontend
yarn install
yarn build

echo "::: Running pkger :::"
cd ../../../../
pkger