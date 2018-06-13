FROM docker.io/mgos/gobase
RUN apt-get update && apt-get install -y zip unzip libftdi-dev libtool libusb-1.0-0
ADD mos /
# allow to mount docker "static" binary from coreos
RUN ln -s libdevmapper.so.1.02.1 /lib/x86_64-linux-gnu/libdevmapper.so.1.02
ENTRYPOINT ["/mos"]
