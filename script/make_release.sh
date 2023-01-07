#!/usr/bin/env bash

set -eu

root=$(git rev-parse --show-toplevel)

function main() {
    apictl
}

main