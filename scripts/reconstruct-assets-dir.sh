#!/bin/sh

set -euo pipefail

REPO_DIR=${1?A git repository must be provided}
CACHE_DIR=${2?A cache repository must be provided}

tmpdir=$(mktemp -d)
index="${tmpdir}/index.yaml"
git -C "${REPO_DIR}" show HEAD:index.yaml > "${index}"

for name in $(yq -re '.entries | keys[]' "${index}"); do
  present=0; cached=0; downloaded=0

  dest_dir="${REPO_DIR}/assets/${name}"
  mkdir -p "${dest_dir}"
  for entry in $(yq -re ".entries[\"${name}\"]" "${index}" | yq -re '.[] | "\(.version)#\(.urls[0])"'); do
    version=$(echo "${entry}" | cut -d'#' -f1)
    url=$(echo "${entry}" | cut -d'#' -f2-)

    chart_filename="${name}-${version}.tgz"
    if [ -f "${dest_dir}/${chart_filename}" ] || git -C "${REPO_DIR}" cat-file -e "HEAD:assets/${name}/${chart_filename}" 2>/dev/null ; then
      present=$((present+1))
    elif [ -f "${CACHE_DIR}/assets/${name}/${chart_filename}" ]; then
      # found in "cache"
      cached=$((cached+1))
      cp "${CACHE_DIR}/assets/${name}/${chart_filename}" "${dest_dir}/${chart_filename}"
    elif git -C "${CACHE_DIR}" show HEAD:"assets/${name}/${chart_filename}" > "${dest_dir}/${chart_filename}" 2>/dev/null ; then
      # found in "cache" - files tracked in git
      cached=$((cached+1))
    else
      downloaded=$((downloaded+1))
      # helm does not allow downloading with a custom filename
      # also, OCI URLs with a digest ignore the version tag and produce an incorrect filename
      helm fetch "${url}" -d "${tmpdir}" >/dev/null
      mv "${tmpdir}"/*.tgz "${dest_dir}/${chart_filename}"
    fi
  done
  echo "Processed ${name}: present=${present},cached=${cached},downloaded=${downloaded}"
done
