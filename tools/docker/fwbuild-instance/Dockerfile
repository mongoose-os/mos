FROM docker.cesanta.com/gobase
RUN apt-get update && apt-get install -y zip unzip
ADD instance /
ENTRYPOINT ["/instance"]
