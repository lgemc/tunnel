#!/bin/bash
set -e

cd /home/lmanrique/Do/tunnel/lambdas

for dir in authorize-connection list-tunnels tunnel-connect tunnel-disconnect tunnel-proxy; do
  echo "Building $dir..."
  cd $dir
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap main.go
  if [ -f bootstrap ]; then
    zip -j ../../build/lambdas/$dir.zip bootstrap
    rm bootstrap
    echo "✓ $dir built"
  else
    echo "✗ $dir failed"
  fi
  cd ..
done

echo "All Lambda functions built successfully!"
