#!/usr/bin/env bash

set -eux

root=$(git rev-parse --show-toplevel)

owner=humanlogio
name=apictl
tag=0.1.0

function list_archive_filenames() {
    jq < dist/artifacts.json -r '.[] | select(.type == "Archive") | .name'
}

function get_archive_url() {
    read filename
    gh api graphql -F owner=${owner} -F name=${name} -F tag=${tag} -F filename=${filename} -f query='
        query($name: String!, $owner: String!, $tag: String!, $filename: String!) {
            repository(name: $name, owner: $owner) {
                release(tagName: $tag) {
                    releaseAssets(first: 1, name: $filename) {
                        nodes {
                            downloadUrl
                        }
                    }
                }
            }
        }' --jq '.data.repository.release.releaseAssets.nodes[0].downloadUrl'
}

function main() {
    list_archive_filenames | get_archive_url | cat
    
}

main 