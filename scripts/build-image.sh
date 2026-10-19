#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:-${IMAGE:-hub.kce.ksyun.com/ksyun-public/sandbox-operator:v20261019}}"

docker build -t "${IMAGE}" .

echo "Built ${IMAGE}"
