#!/usr/bin/env bash

set -euo pipefail

GOOS="linux" go build -ldflags='-s -w' -o linux/amd64/bin/main github.com/paketo-buildpacks/native-image/v5/cmd/main
GOOS="linux" GOARCH="arm64" go build -ldflags='-s -w' -o linux/arm64/bin/main github.com/paketo-buildpacks/native-image/v5/cmd/main

if [ "${STRIP:-false}" != "false" ]; then
  strip linux/amd64/bin/main linux/arm64/bin/main
fi

if [ "${COMPRESS:-none}" != "none" ]; then
  $COMPRESS linux/amd64/bin/main linux/arm64/bin/main
fi

ln -fs main linux/amd64/bin/build
ln -fs main linux/arm64/bin/build
ln -fs main linux/amd64/bin/detect
ln -fs main linux/arm64/bin/detect
