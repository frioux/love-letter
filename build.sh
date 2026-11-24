#!/bin/sh

set -e

(cd fe; npm install; npm run build)

# The Pi Zero is ARMv6; newer Go toolchains default to ARMv7 when
# cross-compiling, which produces a binary that dies with SIGILL.
GOOS=linux GOARCH=arm GOARM=6 go build .
