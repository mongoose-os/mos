FROM ubuntu:jammy

# https://docs.docker.com/engine/install/ubuntu/#install-using-the-repository
RUN apt-get update && \
    apt-get -y install ca-certificates curl gnupg lsb-release && \
    mkdir -p /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list && \
    apt-get update && \
    apt-get -y install docker-ce docker-ce-cli git libftdi1-2 libusb-1.0-0 && \
    apt-get clean

ADD mos /usr/local/bin

ENTRYPOINT ["/usr/local/bin/mos"]
