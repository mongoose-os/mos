#!/bin/bash

PACKAGE=$1
DISTR=$2
VERSION=$3
SRC=$PWD

[ -z "${PACKAGE}" -o -z "${DISTR}" ] && { echo "Usage: $0 <package> <distr> [<version>]"; exit 1; }

RECIPE="$SRC/mos/ubuntu/${PACKAGE}-${DISTR}.recipe"

if [ -n "${VERSION}" ]; then
  # Version is given: replace 1.11_CHANGE_ME with the actual version
  RECIPE_TMP_DIR="$HOME/tmp/recipe-${PACKAGE}-${DISTR}-${VERSION}"
  mkdir -p ${RECIPE_TMP_DIR}
  cat ${RECIPE} | sed "s/1\.11_CHANGE_ME/${VERSION}/" > ${RECIPE_TMP_DIR}/recipe
  RECIPE="${RECIPE_TMP_DIR}/recipe"
fi

set -x -e

IMAGE=docker.io/mgos/ubuntu-golang:${DISTR}

if ! [ -d "$SRC/vendor/github.com" ]; then
  make deps
fi

mkdir -p $HOME/tmp

rm -rf $HOME/tmp/out-${DISTR}/*
docker pull ${IMAGE}
docker run -i -t --rm \
    -v $SRC:/src \
    -v ${RECIPE}:/recipe \
    -v $HOME/tmp/out-${DISTR}:/tmp/work \
    -e HOME=/tmp \
    --user $(id -u):$(id -g) \
    ${IMAGE} \
    /bin/bash -l -c "\
        cd /src && \
        git config --global user.name dummy && git config --global user.email dummy && \
        git-build-recipe --allow-fallback-to-native --package ${PACKAGE} --distribution ${DISTR} \
            /recipe /tmp/work && \
        cd /tmp/work/${PACKAGE} && \
        debuild --no-tgz-check -us -uc -b"
