#!/bin/sh

set -e

(cd fe; npm run build)
GOOS=linux GOARCH=arm go build .
