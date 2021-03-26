FROM golang:1 AS builder

WORKDIR /go/src/bestdrbdmodule
COPY . .
RUN make && mv ./bestdrbdmodule /

FROM ubuntu:focal
COPY --from=builder /bestdrbdmodule /sbin
RUN apt-get update && \
	apt-get install -y python3-pip && \
	pip3 install https://github.com/LINBIT/python-lbdist/archive/master.tar.gz && \
	apt-get -y clean

EXPOSE 3030
CMD ["-addr", ":3030"]
ENTRYPOINT ["/sbin/bestdrbdmodule"]
