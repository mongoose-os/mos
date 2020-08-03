#!/bin/bash

PACKAGE=$1
DISTR=$2
VERSION=$3
SRC=$PWD

[ -z "${PACKAGE}" -o -z "${DISTR}" ] && { echo "Usage: $0 <package> <distr> [<version>]"; exit 1; }

RECIPE="$SRC/tools/ubuntu/${PACKAGE}-${DISTR}.recipe"
IMAGE="docker.io/mgos/ubuntu-golang:${DISTR}"

if [ -n "${VERSION}" ]; then
  # Version is given: replace 1.11_CHANGE_ME with the actual version
  RECIPE_TMP_DIR="$HOME/tmp/recipe-${PACKAGE}-${DISTR}-${VERSION}"
  mkdir -p ${RECIPE_TMP_DIR}
  cat ${RECIPE} | sed "s/1\.11_CHANGE_ME/${VERSION}/" > ${RECIPE_TMP_DIR}/recipe
  RECIPE="${RECIPE_TMP_DIR}/recipe"
fi

set -x -e

make deps

mkdir -p $HOME/tmp/out-${DISTR}
rm -rf $HOME/tmp/out-${DISTR}/*
docker pull ${IMAGE}

DOCKER_RUN_FLAGS="-i -t --rm \
  -v $SRC:/src \
  -v ${RECIPE}:/recipe \
  -v $HOME/tmp/out-${DISTR}:/tmp/work \
  -e HOME=/tmp/work \
  --user=$(id -u):$(id -g)"

docker run ${DOCKER_RUN_FLAGS} ${IMAGE} \
    /bin/bash -l -c "\
      cd /src && \
      git config --global user.name dummy && git config --global user.email dummy && \
      git-build-recipe --allow-fallback-to-native --package ${PACKAGE} --distribution ${DISTR} /recipe /tmp/work \
    "

# Build binary package with no network access top make sure we got all the depndencies we need.
docker run ${DOCKER_RUN_FLAGS} --network=none ${IMAGE} \
    /bin/bash -l -c "\
      cd /tmp/work/${PACKAGE} && \
      debuild --no-tgz-check -us -uc -b \
    "
