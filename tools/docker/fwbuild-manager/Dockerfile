FROM ubuntu:bionic
RUN echo '{}' > /root/.dockercfg
ADD fwbuild-manager /usr/local/bin
ENTRYPOINT ["/usr/local/bin/fwbuild-manager"]
