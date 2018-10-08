FROM golang:1.11-stretch as builder
WORKDIR /go/src/github.com/moov-io/paygate
RUN apt-get update && apt-get install make gcc g++
RUN adduser -q --gecos '' --disabled-login --shell /bin/false moov
USER moov
COPY . .
RUN make build

FROM debian:9
RUN apt-get update && apt-get install -y ca-certificates
COPY --from=builder /go/src/github.com/moov-io/paygate/bin/paygate /bin/paygate
COPY --from=builder /etc/passwd /etc/passwd
USER moov
VOLUME "/data"
EXPOSE 8080
EXPOSE 9090
ENTRYPOINT ["/bin/paygate"]
