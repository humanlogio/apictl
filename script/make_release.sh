#!/usr/bin/env bash

set -eu

root=$(git rev-parse --show-toplevel)

owner=humanlogio
name=apictl
tag=0.1.0

function list_archives() {
    jq < dist/artifacts.json -r '.[] | select(.type == "Archive") | .name, .path, .goos, .goarch'
}

function handle_archive() {
    while read -r filename; read -r path; read -r goos; read -r goarch; do
        local url=$(get_archive_url ${filename})
        if [ -z "${url}" ]; then continue; fi
        local sig=$(get_signature ${path})

        ./apictl create version-artifact \
            --major $(get_version_major) \
            --minor $(get_version_minor) \
            --patch $(get_version_patch) \
            --sha256 $(get_sha256sum ${path}) \
            --url ${url} \
            --os ${goos} \
            --arch ${goarch} \
            --sig ${sig}
    done
}

function get_archive_url() {
    local filename=${1}
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

function get_sha256sum() {
    local filename=${1}
    shasum -a 256 ${filename} | cut -d " " -f 1
}

function get_signature() {
    local filename=${1}
    cat ${filename}.sig
}

function get_version_major() {
    major=`echo ${tag} | cut -d. -f1`
    echo "${major}"
}

function get_version_minor() {
    minor=`echo ${tag} | cut -d. -f2`
    echo "${minor}"
}

function get_version_patch() {
    patch=`echo ${tag} | cut -d. -f3`
    echo "${patch}"
}

function main() {
    list_archives | handle_archive
}

main 