FROM ubuntu:bionic
RUN apt-get update && apt-get install -y zip unzip && apt-get clean
ADD fwbuild-instance /usr/local/bin
ENTRYPOINT ["/usr/local/bin/fwbuild-instance"]
