FROM golang:1.16-buster as builder
WORKDIR /go/src/github.com/moov-io/paygate
RUN apt-get update && apt-get upgrade -y && apt-get install make gcc g++
COPY . .
RUN make build

FROM debian:stable-slim
LABEL maintainer="Moov <support@moov.io>"
RUN apt-get update && apt-get upgrade -y && apt-get install -y ca-certificates
COPY --from=builder /go/src/github.com/moov-io/paygate/bin/paygate /bin/paygate

VOLUME "/data"
ENV SQLITE_DB_PATH /data/paygate.db
# RUN adduser -q --gecos '' --disabled-login --shell /bin/false moov
# RUN chown -R moov: /data
# USER moov

EXPOSE 8080
EXPOSE 9090
ENTRYPOINT ["/bin/paygate"]
