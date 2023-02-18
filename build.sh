#!/bin/sh

set -e

(cd fe; npm run build)
go build .
