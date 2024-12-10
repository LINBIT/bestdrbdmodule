FROM golang:1 AS builder

WORKDIR /go/src/bestdrbdmodule
COPY . .
RUN make && mv ./bestdrbdmodule /

FROM python:3
COPY --from=builder /bestdrbdmodule /sbin
RUN pip3 install https://github.com/LINBIT/python-lbdist/archive/master.tar.gz

EXPOSE 3030
CMD ["-addr", ":3030"]
ENTRYPOINT ["/sbin/bestdrbdmodule"]
