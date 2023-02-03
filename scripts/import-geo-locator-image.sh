#!/bin/bash

echo "Build geo-locator image"
docker build . -t geo-locator

echo "Generate tar file"
docker save --output geo-locator.tar geo-locator

echo "Import in K3s cluster"
k3s ctr images import geo-locator.tar

echo "Clean tar file"
rm -rf geo-locator.tar