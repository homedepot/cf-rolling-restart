#!/bin/bash
set -ex

PROJECT_DIR="$(cd "$(dirname "$0")/.."; pwd)"

go build -o ${PROJECT_DIR}/bin/osx/rolling-restart-plugin-darwin
cf uninstall-plugin RollingRestartPlugin || true
cf install-plugin ${PROJECT_DIR}/bin/osx/rolling-restart-plugin-darwin -f